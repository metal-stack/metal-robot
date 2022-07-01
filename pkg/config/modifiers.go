package config

type Modifier struct {
	Type string         `json:"type" description:"name of the modifier"`
	Args map[string]any `json:"args" description:"modifier configuration"`
}

type LinePatchConfig struct {
	File            string  `mapstructure:"file" description:"the name of the file to be patched"`
	Line            int     `mapstructure:"line" description:"the line number in the file to be patched"`
	ReplaceTemplate *string `mapstructure:"template" description:"a special template to be used for patching the line"`
}

type YAMLPathPatchConfig struct {
	File           string  `mapstructure:"file" description:"the name of the file to be patched"`
	YAMLPath       string  `mapstructure:"yaml-path" description:"the yaml path to the version"`
	Template       *string `mapstructure:"template" description:"a special template to be used for patching the version"`
	VersionCompare *bool   `mapstructure:"version-compare" description:"makes a version comparison before replacement and only replaces if version is greater than current"`
}
