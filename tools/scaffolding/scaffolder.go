package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	defaultSourceControlProvider = "github.com"
	defaultRepoName              = "clutch-custom-gateway"
)

type workflowTemplateValues struct {
	Name           string
	PackageName    string
	Description    string
	DeveloperName  string
	DeveloperEmail string
	URLRoot        string
	URLPath        string
}

type gatewayTemplateValues struct {
	RepoOwner    string
	RepoName     string
	RepoProvider string
}

func promptOrDefault(prompt string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	if defaultValue == "" {
		fmt.Printf("%s: ", prompt)
	} else {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	}
	text, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	text = strings.TrimSpace(text)

	if text == "" && defaultValue != "" {
		return defaultValue
	}
	if text == "" {
		log.Fatal("no input provided")
	}

	return text
}

func determineGoPath() string {
	goPathOut, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(string(goPathOut))
}

func determineUsername() string {
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	return u.Username
}

func determineUserEmail() string {
	gitEmail, err := exec.Command("git", "config", "user.email").Output()
	if err != nil {
		log.Fatal(err)
	}
	email := strings.TrimSpace(string(gitEmail))
	if email == "" {
		email = "unknown@example.com"
	}
	return email
}

func getFrontendPluginTemplateValues() (*workflowTemplateValues, string) {
	log.Println("Welcome!")
	fmt.Println("*** Analyzing environment...")

	dest := filepath.Join(os.Getenv("OLDPWD"), "frontend", "workflows")

	fmt.Println("\n*** Based on your environment, we've picked the following destination for your new workflow:")
	fmt.Println(">", dest)
	okay := promptOrDefault("Is this okay?", "Y/n")
	if !strings.HasPrefix(strings.ToLower(okay), "y") {
		dest = promptOrDefault("Enter the destination folder", dest)
	}
	data := &workflowTemplateValues{}
	data.Name = strings.Title(promptOrDefault("Enter the name of this workflow", "Hello World"))
	description := promptOrDefault("Enter a description of the workflow", "Greet the world")
	data.Description = strings.ToUpper(description[:1]) + description[1:]
	data.DeveloperName = promptOrDefault("Enter the developer's name", determineUsername())
	data.DeveloperEmail = promptOrDefault("Enter the developer's email", determineUserEmail())

	// n.b. transform workflow name into package name, e.g. foo bar baz -> fooBarBaz
	packageName := strings.ToLower(data.Name[:1]) + strings.Title(data.Name)[1:]
	data.PackageName = strings.Replace(packageName, " ", "", -1)

	data.URLRoot = strings.Replace(strings.ToLower(data.Name), " ", "", -1)
	data.URLPath = "/"

	return data, filepath.Join(dest, data.PackageName)
}

func getGatewayTemplateValues() (*gatewayTemplateValues, string) {
	// Ask the user if assumptions are correct or a new destination is needed.
	log.Println("Welcome!")
	fmt.Println("*** Analyzing environment...")

	gopath := determineGoPath()
	username := determineUsername()

	fmt.Println("> GOPATH:", gopath)
	fmt.Println("> User:", username)

	dest := filepath.Join(gopath, "src", defaultSourceControlProvider, username, defaultRepoName)
	fmt.Println("\n*** Based on your environment, we've picked the following destination for your new repo:")
	fmt.Println(">", dest)
	fmt.Println("\nNote: please pay special attention to see if the username matches your provider's username.")
	okay := promptOrDefault("Is this okay?", "Y/n")
	data := &gatewayTemplateValues{
		RepoOwner:    username,
		RepoName:     defaultRepoName,
		RepoProvider: defaultSourceControlProvider,
	}
	if !strings.HasPrefix(strings.ToLower(okay), "y") {
		data.RepoProvider = promptOrDefault("Enter the name of the source control provider", data.RepoProvider)
		data.RepoOwner = promptOrDefault("Enter the name of the repository owner or org", data.RepoOwner)
		data.RepoName = promptOrDefault("Enter the desired repository name", data.RepoName)
		dest = promptOrDefault(
			"Enter the destination folder",
			filepath.Join(gopath, "src", data.RepoProvider, data.RepoOwner, data.RepoName),
		)
	}

	return data, dest
}

func generateAPI(dest string) {
	// TODO: Move this to occur in tmpdir once clutch is published publicly.
	log.Println("Generating API code from protos...")
	log.Println("cd", dest, "&& make api")
	if err := os.Chdir(dest); err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command("make", "api")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		log.Fatal("`make api` in the destination dir returned the above error")
	}
	log.Println("API generation complete")

	fmt.Println("*** All done!")
	fmt.Println("\n*** Try the following command to get started developing the custom gateway:")
	fmt.Println("cd", dest, "&& make")
}

func generateFrontend(dest string) {
	// Update clutch.config.js for new workflow
	log.Println("Compiling workflow, this may take a few minutes...")
	log.Println("cd", dest, "&& yarn --frozen-lockfile && yarn tsc && yarn compile")
	if err := os.Chdir(dest); err != nil {
		log.Fatal(err)
	}

	installCmd := exec.Command("yarn", "--frozen-lockfile")
	if out, err := installCmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		log.Fatal("`yarn --frozen-lockfile` returned the above error")
	}

	compileTypesCmd := exec.Command("yarn", "tsc")
	if out, err := compileTypesCmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		log.Fatal("`yarn tsc` returned the above error")
	}

	compileDevCmd := exec.Command("yarn", "compile")
	if out, err := compileDevCmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		log.Fatal("`yarn compile` returned the above error")
	}

	frontendDir := filepath.Join(os.Getenv("OLDPWD"), "frontend")
	log.Println("Moving to", frontendDir)
	if err := os.Chdir(frontendDir); err != nil {
		log.Fatal(err)
	}
	log.Println("Registering workflow...")
	log.Println("yarn workspace @clutch-sh/app register-workflows")

	registerCmd := exec.Command("yarn", "workspace", "@clutch-sh/app", "register-workflows")
	if out, err := registerCmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		log.Fatal("yarn workspace @clutch-sh/app register-workflows")
	}
	log.Println("Frontend generation complete")

	fmt.Println("*** All done!")
	fmt.Println("\n*** Try the following command to get started developing the new workflow:")
	fmt.Println("make frontend-dev")
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	mode := flag.String("m", "gateway", "oneof gateway, workflow")

	flag.Parse()
	// Collect info from user based on mode and determine template root.
	var dest string
	var templateRoot string
	var data interface{}
	var postProcessFunction func(dest string)

	switch *mode {
	case "gateway":
		templateRoot = filepath.Join(root, "templates/gateway")
		data, dest = getGatewayTemplateValues()
		postProcessFunction = generateAPI
	case "frontend-plugin":
		templateRoot = filepath.Join(root, "templates/frontend")
		data, dest = getFrontendPluginTemplateValues()
		postProcessFunction = generateFrontend
	default:
		flag.Usage()
		os.Exit(1)
	}

	// Check if dest exists.
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		log.Fatal("ERROR destination folder exists")
	}

	fmt.Println("\n*** Generating...")
	log.Println("Using templates in", templateRoot)

	// Make a tmpdir for output.
	tmpout, err := ioutil.TempDir(os.TempDir(), "clutch-scaffolding-*")
	if err != nil {
		log.Fatal("could not create temp dir", err)
	}
	defer os.RemoveAll(tmpout)
	log.Println("Using tmpdir", tmpout)

	// Walk files and template them.
	err = filepath.Walk(templateRoot, func(path string, info os.FileInfo, err error) error {
		relpath, err := filepath.Rel(templateRoot, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if relpath != "." {
				err := os.MkdirAll(filepath.Join(tmpout, relpath), 0755)
				return err
			}
			return nil
		}
		log.Println(relpath)

		t, err := template.ParseFiles(path)
		if err != nil {
			return err
		}

		fh, err := os.Create(filepath.Join(tmpout, relpath))
		if err != nil {
			return err
		}

		return t.Execute(fh, data)
	})

	if err != nil {
		log.Fatal(err)
	}

	// Move tmpdir contents to destination.
	log.Println("Moving to", dest)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.Rename(tmpout, dest); err != nil {
		if os.IsExist(err) {
			log.Fatal("destination folder already exists")
		}
		log.Fatal(err)
	}

	postProcessFunction(dest)
}
