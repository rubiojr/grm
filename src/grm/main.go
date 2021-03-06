package main

import (
	"github.com/jawher/mow.cli"
	"log"
	"strings"
	"crypto/sha256"
	"os/user"
	"path/filepath"
	"os"
	"fmt"
	"bufio"
	"golang.org/x/crypto/ssh/terminal"
	"syscall"
	"crypto/cipher"
	"crypto/aes"
	"encoding/base64"
	"io"
	"crypto/rand"
	"github.com/google/go-github/github"
	"time"
	"grm/config"
	"github.com/denisbrodbeck/machineid"
)

var (
	homeDir       *string
	verbose       *bool
	machineKey    []byte
	configuration config.Configuration
	buildVersion  = "unknown"
	buildDate     = "unknown"
)

func main() {
	app := cli.App("grm", "Github Release Monitor")

	verbose = app.BoolOpt("v verbose", false, "Verbose logging mode")
	homeDir = app.StringOpt("h home", readUserHome(), "Specify a base directory for the configuration, default: current user's home")
	machineKey = generateMachineKey()

	app.Version("version", fmt.Sprintf("Github-Release-Monitor (GRM)\nGit Revision %s (Date: %s UTC)", buildVersion, buildDate))

	app.Before = func() {
		configuration = config.NewConfiguration(*homeDir)
	}

	app.Command("report", "Generates a release report for the remote Github users", cmdReport)
	app.Command("auth", "Configures authorization credentials for remote Github users", cmdAuth)
	app.Command("remote", "Configures remote Github user definitions", cmdRemote)
	app.Command("config", "Sets, gets configuration properties for remote Github users", cmdConfig)
	app.Command("export", "Exports configuration properties for remote Github users", cmdExport)
	app.Command("import", "Imports configuration properties for remote Github users", cmdImport)
	app.Command("license", "Prints all license information for vendored dependencies", cmdLicenses)

	app.Run(os.Args)
}

func readUserHome() string {
	user, err := user.Current()
	if err != nil {
		log.Fatal("Cannot retrieve current user: ", err)
	}

	homeDir := user.HomeDir
	homeDir, err = filepath.Abs(homeDir)
	if err != nil {
		log.Fatal("Cannot retrieve current user's homedir: ", err)
	}

	return homeDir
}

func generateMachineKey() []byte {
	machineId, err := machineid.ID()
	if err != nil {
		log.Fatal("Could not generate machine id: ", err)
	}
	data := []byte(machineId)
	hash := sha256.Sum256(data)
	return hash[:]
}

func readLine(text string, hide bool, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print(fmt.Sprintf("%s ", text))

	var (
		dat  []byte
		line string
		err  error
	)

	if !hide {
		line, err = reader.ReadString('\n')
	} else {
		dat, err = terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		if err == nil {
			line = string(dat)
		}
	}

	if err != nil {
		log.Fatal("Could not read input from terminal: ", err)
	}

	line = strings.Replace(line, "\r", "", -1)
	line = strings.TrimSpace(line)

	if line == "" {
		return defaultValue
	}
	return line
}

func readYesNoQuestion(text string, defaultsToYes bool) bool {
	if defaultsToYes {
		text = fmt.Sprintf("%s [Yes|no] ", text)
	} else {
		text = fmt.Sprintf("%s [yes|No] ", text)
	}

	defaultValue := "no"
	if defaultsToYes {
		defaultValue = "yes"
	}

	line := readLine(text, false, defaultValue)
	line = strings.ToLower(line)

	if line == "yes" || line == "y" || line == "true" || line == "1" {
		return true
	}
	return false
}

func encrypt(value string, key []byte) (string, string) {
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal("Could not setup password encryption: ", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Fatal("Could not setup password encryption: ", err)
	}

	salt := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		log.Fatal("Could not generate a unique password salt: ", err)
	}

	encrypted := aesgcm.Seal(nil, salt, []byte(value), nil)
	return base64.StdEncoding.EncodeToString(encrypted), base64.StdEncoding.EncodeToString(salt)
}

func decrypt(value, salt string, key []byte) string {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		log.Fatal("Could not decryption password: ", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal("Could not setup password decryption: ", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Fatal("Could not setup password decryption: ", err)
	}

	iv, err := base64.StdEncoding.DecodeString(salt)
	if err != nil {
		log.Fatal("Could not decode the password salt: ", err)
	}

	decrypted, err := aesgcm.Open(nil, iv, data, nil)
	if err != nil {
		log.Fatal("Could not decrypt password: ", err)
	}

	return string(decrypted)
}

func rateLimit(response *github.Response) bool {
	if response.Remaining > 0 {
		return false
	}

	delta := time.Now().UTC().Unix() - response.Reset.Unix()
	time.Sleep(time.Duration(delta) * time.Nanosecond)
	return true
}

func hasMorePages(response *github.Response) bool {
	return response.NextPage != 0
}
