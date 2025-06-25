package swaggerctl

import (
	"github.com/Interhyp/go-backend-service-common/acorns/controller"
	"github.com/StephanHCB/go-autumn-acorn-registry/api"
)

// --- implementing Acorn ---

func New(additionalSpecFiles ...controller.SpecFile) auacornapi.Acorn {
	return &SwaggerCtlImpl{
		additionalSpecFiles: additionalSpecFiles,
	}
}

// NewNoAcorn performs the full Acorn lifecycle for this component, no further setup necessary
func NewNoAcorn(additionalSpecFiles ...controller.SpecFile) controller.SwaggerController {
	return &SwaggerCtlImpl{
		additionalSpecFiles: additionalSpecFiles,
	}
}

func (a *SwaggerCtlImpl) IsSwaggerController() bool {
	return true
}

func (a *SwaggerCtlImpl) AcornName() string {
	return controller.SwaggerControllerAcornName
}

func (a *SwaggerCtlImpl) AssembleAcorn(registry auacornapi.AcornRegistry) error {
	return nil
}

func (a *SwaggerCtlImpl) SetupAcorn(registry auacornapi.AcornRegistry) error {
	return nil
}

func (a *SwaggerCtlImpl) TeardownAcorn(registry auacornapi.AcornRegistry) error {
	return nil
}
