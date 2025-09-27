# testgen

**Proof-of-concept: AI-powered Go test generation tool**  
[Repository link](https://github.com/Eranmonnie/testgen)

---

## 🚧 Status

This project is **in development** and may contain bugs or rough edges. It works most of the time, but expect some weirdness! Pull requests, feedback, and ideas are welcome.

## ✨ What is it?

**testgen** is a CLI tool that automatically generates Go tests for your project using AI.  
It aims to save you from the most tedious part of Go development: writing unit tests.  
You can run it manually, or wire it into your git workflow for automatic test generation.

> _"Was tired of writing tests, so I'm making a tool to solve that on the fly."_

## 🛠️ Features

- **AI-powered test generation:** Uses OpenAI, Anthropic, Groq, or local models to generate tests.
- **Git integration:** Analyze recent changes, specific files, or functions.
- **Configurable filtering:** Control which functions get tested.
- **Hooks support:** (Optional) Install git hooks for auto mode.
- **Customizable settings:** YAML config file for easy tweaks.
- **Dry-run and verbose output:** Preview actions before committing.
- **Backups and overwrite protection:** Doesn't clobber your work.

## ⚡ Usage

### 1. Install & Initialize

```sh
go install github.com/Eranmonnie/testgen/cmd/testgen@latest
testgen init
```
- This creates a `.testgen.yml` config file.
- Optionally, set up git hooks for auto mode.

### 2. Set your API key

```sh
export TESTGEN_API_KEY=your_openai_key
```
Supports multiple providers: `openai`, `anthropic`, `groq`, or `local`.

### 3. Generate tests!

```sh
testgen generate                 # Analyze recent git changes
testgen generate user.go         # Specific file(s)
testgen generate --range HEAD~3..HEAD # Specific git range
testgen generate --function ValidateUser # Specific function
```

### 4. Advanced

- Edit `.testgen.yml` to customize filtering, templates, and provider.
- Use `--dry-run` and `--verbose` flags for safe previewing.

## 🧩 Configuration

Your `.testgen.yml` lets you tweak:
- AI provider/model (OpenAI, etc.)
- Filtering rules (skip patterns, complexity, parameters, etc.)
- Overwrite/backup behavior
- Custom test templates

## 🪛 Commands

- `testgen init` — Set up config and hooks
- `testgen generate [files...]` — Generate tests for files/changes/functions
- `testgen config` — Manage configuration
- `testgen hooks install` — Install git hooks (optional)
- `testgen status` — Show hooks/config status

## 🐞 Bugs & Limitations

- This is a work-in-progress—expect bugs, especially with complex code.
- AI-generated tests may require review and tweaks.
- Only Go is supported for now.

## 📚 Example Test Output

```go
func TestValidateUser_ValidUser(t *testing.T) {
    // test code
}
```

## 💡 Why?

Because writing tests is important, but boring. Let the robots do it.

## 🏗️ Contributing

PRs, issues, and suggestions are very welcome!  
If you hit a bug or want a feature, open an issue.

---

**License:** _TBD_

**Author:** [Eranmonnie](https://github.com/Eranmonnie)
