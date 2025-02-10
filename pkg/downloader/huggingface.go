package downloader

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/opea-project/GenAIInfra/opea-operator/api/v1alpha1"
)

const HuggingFaceCliCmd = "huggingface-cli"
const CacheDir = "/data"

func init() {
	huggingfaceCmd.Flags().String("json", "",
		"put hugging face configuration in json format, such as: {\"token\":\"<token>\",\"repoID\":\"<repoID>\"}")
}

var huggingfaceCmd = &cobra.Command{
	Use:   "huggingface",
	Short: "Download models from Hugging Face",
	Long:  `This command allows you to download models from Hugging Face by providing model IDs and options.`,
	Run:   huggingFaceRun,
}

func huggingFaceRun(cmd *cobra.Command, args []string) {

	jsonStr, err := cmd.Flags().GetString("json")
	if err != nil {
		log.Fatalf("Failed to get json flag: %v", err)
	}

	hfConfs := &[]v1alpha1.HuggingFaceModel{}
	if jsonStr != "" {
		if err := json.Unmarshal([]byte(jsonStr), hfConfs); err != nil {
			logrus.Fatalf("Invalid JSON format: %v", err)
		}
	}

	for _, hfConf := range *hfConfs {
		if hfConf.Token == "" || hfConf.RepoID == "" {
			logrus.Fatalf("token and repoID are required")
			continue
		}
		hfCli := NewHuggingFaceCli(&hfConf)
		if err := hfCli.Download(); err != nil {
			logrus.Fatalf("download failed: %v", err)
		}
	}
}

type HuggingFaceCli struct {
	*v1alpha1.HuggingFaceModel
}

func NewHuggingFaceCli(h *v1alpha1.HuggingFaceModel) *HuggingFaceCli {
	return &HuggingFaceCli{h}
}

func (h *HuggingFaceCli) Download() (err error) {

	if err := h.login(); err != nil {
		return err
	}

	downloadCmd := exec.Command(HuggingFaceCliCmd, "download", "--cache-dir", CacheDir, h.RepoID)
	downloadCmd.Stdout = os.Stdout
	downloadCmd.Stderr = os.Stderr
	logCmd(downloadCmd)
	if err = downloadCmd.Run(); err != nil {
		logrus.Errorf("huggface-cli download %v failed: %v", h.RepoID, err)
		return err
	}

	err = h.changeModelMode()
	return err
}

func (h *HuggingFaceCli) login() (err error) {
	loginCmd := exec.Command(HuggingFaceCliCmd, "login", "--token", h.Token)
	loginCmd.Stdout = os.Stdout
	loginCmd.Stderr = os.Stderr
	logCmd(loginCmd)
	if err = loginCmd.Run(); err != nil {
		logrus.Errorf("huggingface-cli login failed: %v", err)
	}
	return err
}

func (h *HuggingFaceCli) changeModelMode() (err error) {
	modePath := fmt.Sprintf("%s/models--%s", CacheDir, strings.ReplaceAll(h.RepoID, "/", "--"))
	chmodCmd := exec.Command("chmod", "-R", "g+w", modePath)
	chmodCmd.Stdout = os.Stdout
	chmodCmd.Stderr = os.Stderr
	logCmd(chmodCmd)
	if err = chmodCmd.Run(); err != nil {
		logrus.Errorf("failed to change mode of %s: %v", modePath, err)
	}
	return err

}

func logCmd(cmd *exec.Cmd) {
	cmdStr := strings.Join(cmd.Args, " ")
	logrus.Infof("Executing command: %v", cmdStr)
}
