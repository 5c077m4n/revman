package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"github.com/jmorganca/ollama/api"
	"github.com/pterm/pterm"
)

const (
	ollamaHost  = "127.0.0.1:11435"
	ollamaModel = "codellama"
)

func getLogPath() (string, error) {
	xdgStateHome, ok := os.LookupEnv("XDG_STATE_HOME")
	if !ok {
		if homeDir, err := os.UserHomeDir(); err != nil {
			return "", err
		} else {
			xdgStateHome = path.Join(homeDir, "state")
		}
	}
	logPath := path.Join(xdgStateHome, "revman")

	if err := os.MkdirAll(logPath, os.ModePerm); err != nil {
		return "", err
	}

	return path.Join(logPath, "general.log"), nil
}

func selectOption(keyValueOptions map[string]any) string {
	options := []string{}
	for compl, desc := range keyValueOptions {
		if desc, ok := desc.(string); ok {
			options = append(options, fmt.Sprintf("%s (%s)", compl, pterm.Green(desc)))
		}
	}

	selectedOption, _ := pterm.DefaultInteractiveSelect.WithOptions(options).Show()
	return selectedOption
}

func main() {
	logPath, _ := getLogPath()
	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	if envErr := os.Setenv("OLLAMA_HOST", ollamaHost); envErr != nil {
		log.Fatalf("failed to set the `OLLAMA_HOST` env var: %s", envErr)
	}

	cmd := exec.Command("ollama", "serve")
	if startCmdErr := cmd.Start(); startCmdErr != nil {
		log.Fatalf("Failed to start cmd: %v", startCmdErr)
	}
	log.Println("Ollama server has started")
	defer cmd.Wait()

	ollamaClient, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	if pullErr := ollamaClient.Pull(
		ctx,
		&api.PullRequest{Model: ollamaModel, Name: ollamaModel},
		func(prog api.ProgressResponse) error {
			log.Printf("%#v\n", prog)
			return nil
		},
	); pullErr != nil {
		log.Fatalf("could not pull the code llama model: %s", err)
	}
	command := strings.Join(os.Args[1:], " ")

	genResp := ""
	if genErr := ollamaClient.Generate(
		ctx,
		&api.GenerateRequest{
			Model: ollamaModel,
			System: strings.Join([]string{
				"You are a CLI completion tool from the UNIX man pages and you show all possible subcommands and flags as key value pairs,",
				"where the key is the subcommand/flag and the value is the description. The commands should be available on the",
				runtime.GOOS,
				"operating system.",
				"Please respond using JSON",
			}, " "),
			Prompt: command,
			Format: "json",
		},
		func(resp api.GenerateResponse) error {
			genResp += resp.Response
			log.Printf("%+v\n", resp)

			return nil
		},
	); genErr != nil {
		log.Fatalf("could not get a response: %v", genErr)
	}
	var respJSON map[string]any
	if unmarshErr := json.Unmarshal([]byte(genResp), &respJSON); unmarshErr != nil {
		log.Fatalf("could not parse Ollama response: %s", unmarshErr)
	}

	selected := selectOption(respJSON)
	fmt.Print(command + " " + strings.Split(selected, "(")[0])
}
