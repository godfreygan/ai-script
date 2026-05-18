import re
import sys
import subprocess

# Files with known compile errors
files = [
    "internal/adapter/litellm.go",
    "internal/adapter/video_adapter.go",
    "internal/middleware/middleware.go",
    "internal/middleware/ratelimit.go",
    "internal/repo/billing.go",
    "internal/repo/migrate.go",
    "internal/repo/pipeline.go",
    "internal/repo/review.go",
    "internal/repo/role.go",
    "internal/repo/script.go",
    "pkg/queue/queue.go",
    "pkg/ws/hub.go",
]

# Pattern: a line starting with // that contains a Go declaration keyword
# We need to split the declaration to its own line.
# Keywords that can appear at package level (after comment):
patterns = [
    (r'^(\s*//.*?)\s*(package\s+\w+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(import\s*\()', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(var\s+\w+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(func\s+\()', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(func\s+\w+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(type\s+\w+)', r'\1\n\2'),
    (r'^(\s*//.*?)\s*(const\s+\w+)', r'\1\n\2'),
    # Also catch return statements merged into comment
    (r'^(\s*//.*?)\s*(return\s+)', r'\1\n\2'),
]

# Also fix truncated string literals in middleware.go
string_fix = re.compile(r'(WithMsg\("[^"]*?)\)\s*$')

for f in files:
    path = f
    try:
        with open(path, 'r', encoding='utf-8') as fh:
            content = fh.read()
    except Exception as e:
        print(f"SKIP {path}: {e}")
        continue

    original = content
    lines = content.split('\n')
    new_lines = []
    changed = False

    for line in lines:
        stripped = line.strip()
        if stripped.startswith('//'):
            fixed = line
            # Try each pattern
            for pat, repl in patterns:
                new_fixed, count = re.subn(pat, repl, fixed)
                if count > 0:
                    fixed = new_fixed
                    changed = True
                    # If replacement introduced newlines, split and process
                    if '\n' in fixed:
                        parts = fixed.split('\n')
                        new_lines.extend(parts)
                        break
            else:
                # No pattern matched, keep as is
                new_lines.append(fixed)
            continue

        # Fix truncated string in middleware.go
        if 'middleware.go' in f:
            new_line, count = string_fix.subn(r'\1"))', line)
            if count > 0:
                line = new_line
                changed = True

        new_lines.append(line)

    if changed:
        new_content = '\n'.join(new_lines)
        with open(path, 'w', encoding='utf-8') as fh:
            fh.write(new_content)
        print(f"FIXED {path}")
    else:
        print(f"OK    {path}")

print("Done.")
