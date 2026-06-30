# yze-go-emptyiface

A [`yze`](https://github.com/gomatic/yze) analyzer (category `modern-go`) enforcing the gomatic Go standard that the empty interface is written as `any`, not `interface{}`. It offers a **mechanical fix** (`yze --fix` rewrites `interface{}` → `any`; `gopls` surfaces it as a quick-fix).

- **Rule:** `yze/emptyiface`
- **Library:** exports `Analyzer` and `Registration` for the [`yze`](https://github.com/gomatic/yze) aggregator and [`stickler`](https://github.com/gomatic/stickler) runner.
- **Binary:** `cmd/yze-go-emptyiface` runs it standalone (`text`/`-json`/`-fix`, and as a `go vet -vettool`).

Built on the [`go-yze`](https://github.com/gomatic/go-yze) framework.
