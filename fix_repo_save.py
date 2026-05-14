#!/usr/bin/env python3
"""批量修复 repo Update 方法中的 Save 并发覆盖问题.
将 Save(obj) 改为 Model(&model.Type{}).Select("*").Omit("created_at").Where("id = ?", obj.ID).Updates(obj)
"""
import re, os, glob

def fix_file(path):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()
    orig = content

    # 匹配 Update 方法中的 Save 调用：r.db.WithContext(ctx).Save(var).Error
    # 需要推断 model 类型（从函数参数中提取）
    # 简单策略：查找 Update 方法的参数类型
    model_type = None
    var_name = None
    m = re.search(r'func \(r \*\w+Repo\) Update\(ctx context\.Context, (\w+) \*(model\.\w+)\)', content)
    if m:
        var_name = m.group(1)
        model_type = m.group(2)

    if model_type and var_name:
        # 替换 Save(var) 为 Model(&model.Type{}).Select("*").Omit("created_at").Where("id = ?", var.ID).Updates(var)
        # 但要排除关联表插入（如 StoryboardStyle）
        pattern = re.compile(
            re.escape(f'r.db.WithContext(ctx).Save({var_name})')
        )
        replacement = f'r.db.WithContext(ctx).Model(&{model_type}{{}}).Select("*").Omit("created_at").Where("id = ?", {var_name}.ID).Updates({var_name})'
        content = pattern.sub(replacement, content)

    if content != orig:
        with open(path, 'w', encoding='utf-8') as f:
            f.write(content)
        return True
    return False

base = os.path.dirname(os.path.abspath(__file__))
repo_dir = os.path.join(base, 'backend', 'internal', 'repo')
fixed = 0
for path in sorted(glob.glob(os.path.join(repo_dir, '*.go'))):
    if fix_file(path):
        print(f'fixed: {os.path.basename(path)}')
        fixed += 1
print(f'Done. Fixed {fixed} files.')
