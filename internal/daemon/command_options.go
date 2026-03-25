package daemon

type CommandOption struct {
	WorkDir string
}

type CommandServiceOption func(*CommandOption)

func CommandServiceOptionWithWorkDir(workDir string) CommandServiceOption {
	return func(o *CommandOption) {
		o.WorkDir = workDir
	}
}

func applyCommandOptions(opts []CommandServiceOption) CommandOption {
	o := CommandOption{
		WorkDir: "/",
	}
	for _, opt := range opts {
		opt(&o)
	}

	return o
}
