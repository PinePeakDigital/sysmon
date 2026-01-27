# GitHub Copilot Instructions for sysmon

## Project Overview

This is a terminal-based system monitor written in Go that displays CPU, memory, GPU, and process information in real-time.

## Commit Message Format

**ALWAYS use Conventional Commits format for all commit messages.**

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

- **feat**: A new feature
- **fix**: A bug fix
- **docs**: Documentation only changes
- **style**: Changes that do not affect the meaning of the code (white-space, formatting, missing semi-colons, etc)
- **refactor**: A code change that neither fixes a bug nor adds a feature
- **perf**: A code change that improves performance
- **test**: Adding missing tests or correcting existing tests
- **build**: Changes that affect the build system or external dependencies
- **ci**: Changes to CI configuration files and scripts
- **chore**: Other changes that don't modify src or test files

### Examples

```
feat(ui): add GPU temperature display
fix(stats): correct memory usage calculation
docs(readme): update installation instructions
refactor(processes): simplify process sorting logic
perf(cpu): optimize per-core usage calculation
```

### Guidelines

- Use lowercase for type and scope
- Keep description concise (72 characters or less)
- Use imperative mood ("add" not "added" or "adds")
- Don't end description with a period
- Scope is optional but recommended for clarity

## Code Style

- Follow Go conventions and best practices
- Use `gofmt` for formatting
- Write clear, self-documenting code
- Add comments for complex logic
- Keep functions focused and concise

## Testing

- Ensure changes work on Linux systems
- Test with and without GPU monitoring tools (nvidia-smi, rocm-smi)
- Verify terminal UI remains responsive and readable
