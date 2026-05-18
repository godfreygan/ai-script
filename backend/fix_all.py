import re
import os
import subprocess

# All backend Go files with compile errors
files = [
    "cmd/worker/main.go",
    "internal/adapter/litellm.go",
    "internal/adapter/video_adapter.go",
    "internal/middleware/middleware.go",
    "internal/middleware/ratelimit.go",
    "internal/middleware/validate.go",
    "internal/repo/billing.go",
    "internal/repo/migrate.go",
    "internal/repo/pipeline.go",
    "internal/repo/review.go",
    "internal/repo/role.go",
    "internal/repo/script.go",
    "internal/repo/seed.go",
    "pkg/queue/queue.go",
    "pkg/ws/hub.go",
]

# Pattern 1: comment line merged with code declaration/statement
# Match lines starting with optional whitespace + // + text, then a Go keyword at the end
comment_code_patterns = [
    # declarations
    (r'^(\s*//.*?)\s*(package\s+\w+)$', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(import\s*\()', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(var\s+\w+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(func\s+\()', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(func\s+\w+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(type\s+\w+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(const\s+\w+)', r'\1\n\2'),
    # statements
    (r'^(\s*//.*?)\s*(return\s+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(if\s+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(for\s+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(switch\s+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(case\s+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(default\s*:\s*)$', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(defer\s+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(go\s+)', r'\1\n\2'),
    # assignment/statements without keywords above
    (r'^(\s*//.*?)\s*([a-zA-Z_]\w*\s*:=\s*)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*([a-zA-Z_]\w*\s*=\s*)', r'\1\n\2'),
]

# Pattern 2: truncated string literals - Chinese strings where closing quote became ?
# Common pattern: "some_chinese? instead of "some_chinese"
string_trunc_pattern = re.compile(r'("[^"]*\?)(\s*[,\)])')
string_trunc_pattern2 = re.compile(r'("[^"]*?\?)(\s*[,\)])')

# Pattern 3: middleware-style string truncation in function calls
middleware_string_pattern = re.compile(r'(WithMsg\("[^"\)]*?)\)\s*\)')
middleware_string_pattern2 = re.compile(r'(WithMsg\("[^"]*?)\)$')

def fix_file(path):
    try:
        with open(path, 'r', encoding='utf-8') as f:
            content = f.read()
    except Exception as e:
        print(f"SKIP {path}: {e}")
        return False

    original = content
    lines = content.split('\n')
    new_lines = []
    changed = False

    for line in lines:
        stripped = line.strip()
        
        # Fix comment+code merge
        if stripped.startswith('//'):
            fixed_line = line
            matched = False
            for pat, repl in comment_code_patterns:
                new_line, count = re.subn(pat, repl, fixed_line)
                if count > 0:
                    fixed_line = new_line
                    matched = True
                    changed = True
            
            if matched and '\n' in fixed_line:
                new_lines.extend(fixed_line.split('\n'))
            else:
                new_lines.append(fixed_line)
            continue

        # Fix truncated string literals
        # Look for strings ending with ? followed by comma or close paren
        new_line = line
        
        # Fix "...?,  or "...?)
        for _ in range(5):  # multiple passes
            tmp, count = string_trunc_pattern.subn(r'"\2', new_line)
            if count == 0:
                break
            new_line = tmp
            changed = True
        
        # Fix middleware WithMsg patterns
        tmp, count = middleware_string_pattern.subn(r'\1"))', new_line)
        if count > 0:
            new_line = tmp
            changed = True
        
        tmp, count = middleware_string_pattern2.subn(r'\1"))', new_line)
        if count > 0:
            new_line = tmp
            changed = True
        
        new_lines.append(new_line)

    if changed:
        new_content = '\n'.join(new_lines)
        with open(path, 'w', encoding='utf-8') as f:
            f.write(new_content)
        print(f"FIXED {path}")
        return True
    else:
        print(f"OK    {path}")
        return False

for f in files:
    fix_file(f)

print("\nDone. Running go build...")
os.chdir('backend')
result = subprocess.run(['go', 'build', './...'], capture_output=True, text=True)
print(result.stdout)
print(result.stderr)
if result.returncode == 0:
    print("BUILD SUCCESS")
else:
    print(f"BUILD FAILED ({result.returncode})")
