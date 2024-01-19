package build

import (
	"embed"
	"io"
	"os"
	"os/exec"

	"log/slog"
)

//go:embed dockerassets
var dockerAssets embed.FS

func DockerBuild(gitRepoDir string, imageName string) error {
	cmd := exec.Command("docker", "build", "-t", imageName, ".")
	cmd.Dir = gitRepoDir
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	slog.Debug(cmd.String())
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func DockerTag(srcImageName, dstImageName string) error {
	cmd := exec.Command("docker", "tag", srcImageName, dstImageName)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	slog.Debug(cmd.String())
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func DockerPush(imageName string) error {
	cmd := exec.Command("docker", "push", imageName)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	slog.Debug(cmd.String())
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func DockerInspect(imageName string) error {
	cmd := exec.Command("docker", "manifest", "inspect", imageName)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	slog.Debug(cmd.String())
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}
