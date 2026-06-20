# Server-Side Component Separation with Go's html/template and embed.FS

## Context
When building single-page apps (SPAs) using lightweight libraries like Alpine.js and Vanilla CSS, the HTML files can grow extremely large and hard to maintain. Rather than adopting heavy frontend frameworks (like React, Vue, or Angular) or fetching components dynamically via JS `fetch` calls (which cause visual flickers and layout shifts), Go's standard library `html/template` provides a robust, zero-runtime-overhead solution.

## Approach
1. **Directory Structure**:
   - Extract UI styles to `ui/style.css`.
   - Extract UI scripting to `ui/app.js`.
   - Put reusable/separated HTML sections into `ui/components/<name>.html` files wrapped in Go's `{{define "<name>"}}...{{end}}` blocks.
2. **Main Page Integration**:
   - Use `{{template "<name>" .}}` inside the main `ui/index.html` to reference the sub-templates.
3. **Go Embedding and Rendering**:
   - Embed everything using `//go:embed ui/*` to ensure both component HTMLs, CSS, and JS files are bundled inside the binary.
   - Compile the templates at startup:
     ```go
     tmpl, err := template.ParseFS(uiFS, "ui/index.html", "ui/components/*.html")
     ```
   - Execute the template on client request:
     ```go
     tmpl.Execute(w, nil)
     ```

## Benefits
- **Developer Experience**: Smaller, focused HTML components and separate JS/CSS files make the codebase clean and modular.
- **Flicker-Free Load**: Components are rendered server-side in a single request, eliminating any client-side template parsing or fetch latency.
- **No Build Step**: Requires no npm, bundlers, or asset compilation pipelines. Standard Go compilation is all that is needed.
