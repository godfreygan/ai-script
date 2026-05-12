package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// NewAsynqHandler 把 asynq 的 pipeline.run 任务桥接到 DAG Runner。
//
// 流程:
//  1. 解析 RunPayload{PipelineID, Input, Overrides}
//  2. 加载 pipeline 行,读取 DAG JSON
//  3. 创建 pipeline_runs 记录(status=running)
//  4. 执行 Runner.Execute,得到 output map
//  5. 把 output 写回 pipeline_runs.output,并把 status 改为 succeeded/failed
func NewAsynqHandler(repos *repo.Repositories, runner *Runner, log *zap.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p RunPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode pipeline.run payload: %w", err)
		}
		if p.PipelineID == 0 {
			return fmt.Errorf("pipeline.run: missing pipeline_id")
		}
		pl, err := repos.Pipeline.Get(ctx, p.PipelineID)
		if err != nil {
			return fmt.Errorf("pipeline.run: load pipeline %d: %w", p.PipelineID, err)
		}
		if len(pl.DAG) == 0 {
			return fmt.Errorf("pipeline.run: pipeline %d has empty dag", p.PipelineID)
		}

		// 创建 run 记录
		inputBytes, _ := json.Marshal(p.Input)
		now := time.Now()
		run := &model.PipelineRun{
			PipelineID:  pl.ID,
			ProjectID:   pl.ProjectID,
			TriggerType: "manual",
			Input:       model.JSON(inputBytes),
			Status:      "running",
			StartedAt:   &now,
		}
		if err := repos.Pipeline.CreateRun(ctx, run); err != nil {
			return fmt.Errorf("pipeline.run: create run: %w", err)
		}

		// 合并 overrides 到 input(模型 id / 全局参数等)
		input := map[string]any{}
		for k, v := range p.Input {
			input[k] = v
		}
		for k, v := range p.Overrides {
			input[k] = v
		}

		log.Info("pipeline.run start",
			zap.Int64("pipeline_id", pl.ID),
			zap.Int64("run_id", run.ID),
			zap.Int("input_keys", len(input)),
		)

		out, err := runner.Execute(ctx, []byte(pl.DAG), input, run.ID)
		if err != nil {
			_ = repos.Pipeline.UpdateRunStatus(ctx, run.ID, "failed", truncate(err.Error(), 1000))
			log.Warn("pipeline.run failed",
				zap.Int64("run_id", run.ID),
				zap.Error(err),
			)
			return err
		}
		if out != nil {
			outBytes, _ := json.Marshal(out)
			_ = repos.Pipeline.UpdateRunOutput(ctx, run.ID, model.JSON(outBytes))
		}
		_ = repos.Pipeline.UpdateRunStatus(ctx, run.ID, "succeeded", "")
		log.Info("pipeline.run done",
			zap.Int64("run_id", run.ID),
		)
		return nil
	}
}
