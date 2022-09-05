package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
)

func withProjectName(name string) func(*loader.Options) {
	return func(lOpts *loader.Options) {
		lOpts.SetProjectName(name, true)
	}
}

func readComposeFile(composeFile string) (project *types.Project, err error) {
	if composeFile == "" {
		return nil, errors.New("composeFile is empty")
	}
	fullPath, err := filepath.Abs(composeFile)
	if err != nil {
		fmt.Println("failed to get absolute path")
		return nil, err
	}
	baseDir := filepath.Dir(fullPath)
	var b []byte
	b, err = os.ReadFile(fullPath)
	if err != nil {
		fmt.Println("failed to read compose file: %w", err)
		return project, err
	}
	var files []types.ConfigFile
	files = append(files, types.ConfigFile{Filename: composeFile, Content: b})
	envMap := make(map[string]string)
	// Read Compose File
	project, err = loader.Load(types.ConfigDetails{
		WorkingDir:  baseDir,
		ConfigFiles: files,
		Environment: envMap,
	}, withProjectName(path.Base(baseDir)))
	for i := range project.Services {
		if project.Services[i].CustomLabels == nil {
			project.Services[i].CustomLabels = map[string]string{
				api.ProjectLabel: project.Name,
			}
		}
		project.Services[i].CustomLabels[api.ProjectLabel] = project.Name
		project.Services[i].CustomLabels[api.ServiceLabel] = project.Services[i].Name
		project.Services[i].CustomLabels[api.VersionLabel] = api.ComposeVersion
		project.Services[i].CustomLabels[api.WorkingDirLabel] = project.WorkingDir
		project.Services[i].CustomLabels[api.ConfigFilesLabel] = strings.Join(project.ComposeFiles, ",")
		project.Services[i].CustomLabels[api.OneoffLabel] = "False"
		project.Services[i].CustomLabels["chief.project"] = project.Name

	}
	return project, err
}

func StartProject(composeFile string) (err error) {
	cli, err := command.NewDockerCli(command.WithStandardStreams())
	if err != nil {
		fmt.Println("failed to create docker cli: %w", err)
		return err
	}
	cli.Initialize(flags.NewClientOptions())

	ctx := context.TODO()
	project, err := readComposeFile(composeFile)
	if err != nil {
		fmt.Println("failed to read compose file: %w", err)
		return err
	}

	createOpts := api.CreateOptions{RemoveOrphans: true, IgnoreOrphans: true, QuietPull: false, Inherit: false, Recreate: api.RecreateDiverged}
	startOpts := api.StartOptions{Project: project, CascadeStop: false, Wait: false, AttachTo: project.ServiceNames()}

	opts := api.UpOptions{Start: startOpts, Create: createOpts}
	fmt.Println("Up options", opts)
	composeService := compose.NewComposeService(cli)
	fmt.Println("Compose project", project)
	err = composeService.Create(ctx, project, createOpts)
	if err != nil {
		fmt.Println("failed to create project: %w", err)
		return err
	}
	fmt.Println("Starting project", composeFile)
	err = composeService.Up(ctx, project, opts)
	return err
}

func main() {
	err := StartProject("docker-compose.yml")
	if err != nil {
		fmt.Println("failed to start project: %w", err)
	}
}
