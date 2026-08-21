package api

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"

	// Registers the generated OpenAPI spec that Swagger UI reads.
	_ "github.com/yeremi777/mlbb-analyzer-service/internal/docs"
)

// docsRoutes mounts Swagger UI and the spec it renders. The spec is generated
// from handler annotations by `make docs`, never hand-edited.
func (s *Server) docsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/index.html", http.StatusMovedPermanently)
	})
	mux.Handle("GET /docs/", httpSwagger.Handler(httpSwagger.URL("/docs/doc.json")))
}
