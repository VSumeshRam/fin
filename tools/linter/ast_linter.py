import ast
import os
import sys

def check_max_tokens(filepath):
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()

    try:
        tree = ast.parse(content, filename=filepath)
    except SyntaxError as e:
        print(f"Syntax error in {filepath}: {e}")
        return False

    has_errors = False
    
    for node in ast.walk(tree):
        if isinstance(node, ast.Call):
            func_name = ""
            if isinstance(node.func, ast.Attribute):
                func_name = node.func.attr
            elif isinstance(node.func, ast.Name):
                func_name = node.func.id

            if func_name in ["create", "generate_content", "Completion", "ChatCompletion"]:
                has_max_tokens = any(kw.arg == "max_tokens" for kw in node.keywords)
                if not has_max_tokens:
                    print(f"ERROR: {filepath}:{node.lineno} - LLM call '{func_name}' missing 'max_tokens' limit.")
                    has_errors = True

    return has_errors

def main():
    target_dir = sys.argv[1] if len(sys.argv) > 1 else "."
    failed = False
    for root, _, files in os.walk(target_dir):
        for file in files:
            if file.endswith(".py"):
                filepath = os.path.join(root, file)
                if check_max_tokens(filepath):
                    failed = True
    
    if failed:
        sys.exit(1)
    else:
        print("CI/CD Linter passed: All LLM calls have max_tokens limit.")
        sys.exit(0)

if __name__ == "__main__":
    main()
