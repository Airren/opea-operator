package downloader

import (
	"encoding/base64"
	"testing"

	"github.com/opea-project/GenAIInfra/opea-operator/api/v1alpha1"
)

var token = "aGZfRkNtUk5PRWJrblRSTXd0RmdydlN6RE9FYk5WSmlUU2ZUQw=="

func decodeToken(token string) string {
	decodeBytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return ""
	}
	return string(decodeBytes)
}

var hfConf = &v1alpha1.HuggingFace{
	RepoID:    "BAAI/bge-base-en-v1.5",
	RepoType:  "",
	FileNames: nil,
	Revision:  "",
	Token:     decodeToken(token),
}

func TestHuggingFaceCli_login(t *testing.T) {
	hf := NewHuggingFaceCli(hfConf)
	err := hf.login()
	t.Logf("login result: %v", err)
}

func TestHuggingFaceCli_Download(t *testing.T) {

	hf := NewHuggingFaceCli(hfConf)
	err := hf.Download()
	t.Logf("download result: %v", err)
}
