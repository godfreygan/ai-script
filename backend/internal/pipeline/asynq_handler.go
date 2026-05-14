package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// NewAsynqHandler 把 asynq 的 pipeline.run 任务桥接到 DAG Runner。
//
// 修复 P0 #4:
//   - 不再 worker 端 CreateRun → service 层已 pre-create + return run.ID
//   - 用 payload.RunID 而非 asynq UUID,前端 pipeline:<id> topic 才能命中
//   - 幂等:run.Status 已 succeeded/failed → 直接返回 nil(asynq retry 不重跑)
//
// 流程:
//  1. 解析 RunPayload{RunID, PipelineID, Input, Overrides}
//  2. 查 run 行,若已终结直接幂等返回
//  3. 加载 pipeline DAG
//  4. 标记 run=running,执行 Runner.Execute
//  5. 写回 output + status=succeeded/failed
func NewAsynqHandler(repos *repo.Repositories, runner *Runner, log *zap.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p RunPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode pipeline.run payload: %w", err)
		}
		if p.RunID == 0 || p.PipelineID == 0 {
			return fmt.Errorf("pipeline.run: missing run_id or pipeline_id")
		}

		// 幂等检查 — asynq retry 同一 task 不应重复执行
		run, err := repos.Pipeline.GetRun(ctx, p.RunID)
		if err != nil {
			return fmt.Errorf("pipeline.run: load run %d: %w", p.RunID, err)
		}
		if run.Status == "succeeded" || run.Status == "failed" || run.Status == "cancelled" {
			log.Info("pipeline.run skipped (already terminal)",
				zap.Int64("run_id", p.RunID),
				zap.String("status", run.Status))
			return nil
		}

		pl, err := repos.Pipeline.Get(ctx, p.PipelineID)
		if err != nil {
			_ = repos.Pipeline.UpdateRunStatus(ctx, p.RunID, "failed", truncate(err.Error(), 1000))
			return fmt.Errorf("pipeline.run: load pipeline %d: %w", p.PipelineID, err)
		}
		if len(pl.DAG) == 0 {
			_ = repos.Pipeline.UpdateRunStatus(ctx, p.RunID, "failed", "empty dag")
			return fmt.Errorf("pipeline.run: pipeline %d has empty dag", p.PipelineID)
		}

		// 从 queued 转 running(StartedAt 可能已 service 层设置,这里只刷状态)
		if run.Status == "queued" {
			_ = repos.Pipeline.UpdateRunStatus(ctx, p.RunID, "running", "")
		}

		// 合并 overrides 到 input
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
