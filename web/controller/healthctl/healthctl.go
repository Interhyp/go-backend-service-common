package healthctl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Interhyp/go-backend-service-common/api"
	"github.com/Interhyp/go-backend-service-common/web/util/media"
	"github.com/go-chi/chi/v5"
	"github.com/go-http-utils/headers"
)

type HealthCtlImpl struct{}

func (c *HealthCtlImpl) WireUp(ctx context.Context, router chi.Router) {
	router.Get("/management/health", c.Health)
	router.Get("/health", c.Health)
	router.Get("/", c.Health)
}

func (c *HealthCtlImpl) Health(w http.ResponseWriter, r *http.Request) {
	up := "UP"
	response := api.HealthComponent{
		Status: &up,
	}
	w.Header().Set(headers.ContentType, media.ContentTypeApplicationJson)
	writeJson(r.Context(), w, response)
}

func writeJson(_ context.Context, w http.ResponseWriter, v interface{}) {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(v)
}
