---
name: "git-commit"
description: "Handles git commit operations with proper message formatting. Invoke when user wants to commit code, submit changes, or create a git commit."
---

# Git Commit Skill

This skill handles git commit operations with best practices for commit message formatting.

## When to Invoke

Invoke this skill when:
- User asks to commit code
- User wants to submit/commit changes
- User mentions "commit" or "提交代码"
- User wants to create a git commit

## Commit Message Format

Follow the Conventional Commits specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | A new feature |
| `fix` | A bug fix |
| `docs` | Documentation only changes |
| `style` | Changes that do not affect the meaning of the code |
| `refactor` | A code change that neither fixes a bug nor adds a feature |
| `perf` | A code change that improves performance |
| `test` | Adding missing tests or correcting existing tests |
| `chore` | Changes to the build process or auxiliary tools |
| `ci` | Changes to CI configuration files and scripts |

## Workflow

1. **Check Status**: Run `git status` to see all changes
2. **Review Diff**: Run `git diff` to understand the changes
3. **Stage Changes**: Use `git add` to stage relevant files
4. **Create Commit**: Use `git commit` with a properly formatted message

## Best Practices

1. **Atomic Commits**: Each commit should represent a single logical change
2. **Clear Messages**: Write clear, concise commit messages
3. **Reference Issues**: Include issue numbers when applicable
4. **Breaking Changes**: Use `!` after type for breaking changes, and explain in footer

## Examples

```bash
# Feature commit
git commit -m "feat: add user authentication module"

# Bug fix with scope
git commit -m "fix(auth): resolve token expiration issue"

# Breaking change
git commit -m "feat(api)!: change response format

BREAKING CHANGE: The API response format has changed from v1 to v2.
All clients need to update their parsing logic."

# With issue reference
git commit -m "fix: resolve memory leak in session manager

Closes #123"
```

## Language Support

- Use the same language as the user's message for commit messages
- If user communicates in Chinese, use Chinese commit messages
- If user communicates in English, use English commit messages
