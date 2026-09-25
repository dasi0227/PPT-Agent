package model

// Slide stores page identity and its historical generation inputs. Current
// authoring content, hierarchy and order live in project files.
type Slide struct {
	ID                   string
	ProjectID            string
	LastExportAt         *int64
	GenerationInputsJSON *string `json:"-"`
}
