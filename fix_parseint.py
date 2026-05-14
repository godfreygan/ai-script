#!/usr/bin/env python3
"""批量修复 handler 中 strconv.ParseInt/Atoi 忽略 error 的问题."""
import re, os, glob

def fix_file(path):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()
    orig = content
    lines = content.split('\n')
    new_lines = []
    i = 0
    while i < len(lines):
        line = lines[i]
        # 匹配:  var, _ := strconv.ParseInt/Atoi(...)
        m = re.search(r'^(\s*)(\w+),\s*_\s*(:=)\s*strconv\.(ParseInt|Atoi)\((.*)\)\s*$', line)
        if m:
            indent = m.group(1)
            varname = m.group(2)
            funcn = m.group(4)
            args = m.group(5)
            # 只处理来自 gin.Context 的输入
            if 'c.' in args:
                new_lines.append(f'{indent}{varname}, err := strconv.{funcn}({args})')
                new_lines.append(f'{indent}if err != nil {{')
                new_lines.append(f'{indent}\tresponse.Fail(c, errcode.ErrParam.Wrap(err))')
                new_lines.append(f'{indent}\treturn')
                new_lines.append(f'{indent}}}')
                i += 1
                continue
        # 匹配 if 内的短声明: if v, _ := strconv.ParseInt(...); cond {
        m2 = re.search(r'(\s*if\s+)(\w+),\s*_\s*(:=)\s*strconv\.(ParseInt|Atoi)\((.*)\)\s*;\s*(.*)\{\s*$', line)
        if m2:
            prefix = m2.group(1)
            varname = m2.group(2)
            funcn = m2.group(4)
            args = m2.group(5)
            cond = m2.group(6)
            if 'c.' in args:
                indent = m2.group(0)[:m2.start(1)]
                # 把 if 短声明拆出来
                new_lines.append(f'{indent}{varname}, err := strconv.{funcn}({args})')
                new_lines.append(f'{indent}if err != nil {{')
                new_lines.append(f'{indent}\tresponse.Fail(c, errcode.ErrParam.Wrap(err))')
                new_lines.append(f'{indent}\treturn')
                new_lines.append(f'{indent}}}')
                new_lines.append(f'{indent}if {cond} {{')
                i += 1
                continue
        new_lines.append(line)
        i += 1

    new_content = '\n'.join(new_lines)
    if new_content != orig:
        with open(path, 'w', encoding='utf-8') as f:
            f.write(new_content)
        return True
    return False

base = os.path.dirname(os.path.abspath(__file__))
handler_dir = os.path.join(base, 'backend', 'internal', 'handler')
fixed = 0
for path in sorted(glob.glob(os.path.join(handler_dir, '*.go'))):
    if fix_file(path):
        print(f'fixed: {os.path.basename(path)}')
        fixed += 1
print(f'Done. Fixed {fixed} files.')
