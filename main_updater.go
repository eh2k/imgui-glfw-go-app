package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/eh2k/imgui-glfw-go-app/imgui-go"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	//"./imgui-go"
	"archive/zip"
	"github.com/google/uuid"
)

var _updateErr error = nil
var _latestVersionUrl = "unknown"
var _latestDownload = ""
var _openUpdatePopup = 0

func HttpGet(url string, user string, password string) (*http.Response, error) {
	basicAuth := func(username, password string) string {
		auth := username + ":" + password
		return base64.StdEncoding.EncodeToString([]byte(auth))
	}

	redirectPolicyFunc := func(req *http.Request, via []*http.Request) error {
		req.Header.Add("Authorization", "Basic "+basicAuth(user, password))
		return nil
	}

	client := &http.Client{
		CheckRedirect: redirectPolicyFunc,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic "+basicAuth(user, password))
	return client.Do(req)
}

func Unzip(src string, dest string) ([]string, error) {

	var filenames []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()

	for _, f := range r.File {

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return filenames, fmt.Errorf("%s: illegal file path", fpath)
		}

		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return filenames, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return filenames, err
		}
		defer outFile.Close()

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}
		defer rc.Close()

		_, err = io.Copy(outFile, rc)

		if err != nil {
			return filenames, err
		}
	}
	return filenames, nil
}

func update(oldVersion string) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	ver := _latestVersionUrl[strings.LastIndex(_latestVersionUrl, "/")+1:]
	url := _latestDownload
	targetFolder := filepath.Join(filepath.Dir(exePath), ver)

	if _, err := os.Stat(targetFolder); os.IsNotExist(err) {

		// orig := targetFolder
		// targetFolder += "_tmp"
		// defer os.Rename(targetFolder, orig)

		if err = os.MkdirAll(targetFolder, os.ModePerm); err != nil {
			return err
		}

		zipFile := filepath.Join(targetFolder, uuid.New().String()+".zip")

		{
			res, err := http.Get(url)
			if err != nil {
				return err
			}

			outFile, err := os.Create(zipFile)

			if err != nil {
				return err
			}
			defer outFile.Close()
			_, err = io.Copy(outFile, res.Body)

			if err != nil {
				return err
			}
		}

		{
			_, err := Unzip(zipFile, targetFolder)

			if err != nil {
				return err
			}

			// err = os.Remove(zipFile)

			// if err != nil {
			// 	osdialog.ShowMessageBox(osdialog.Error, osdialog.Ok, err.Error())
			// 	return
			// }
		}
	}

	{

		fmt.Println(exePath)
		newExe := filepath.Join(filepath.Dir(exePath), ver, filepath.Base(exePath))

		filepath.Walk(filepath.Join(filepath.Dir(exePath), ver), func(name string, info os.FileInfo, err error) error {
			if strings.HasSuffix(name, "exe") {
				newExe = name
			}
			return nil
		})
		exec.Command(newExe, os.Args[1:]...).Start()
		os.Exit(0)

		_openUpdatePopup = 0
	}

	return nil
}

func ShowUpdatePopup(version string, projectUrl string) {
	if _latestVersionUrl == "unknown" {
		_latestVersionUrl = version

		go func() {
			releasesUrl := strings.Replace(projectUrl, "https://github.com", "https://api.github.com/repos", 1) + "/releases"
			fmt.Println(releasesUrl)

			r, err := http.Get(releasesUrl) // HttpGet(releasesUrl, "usr", "pwd")
			if err != nil {
				return
			}

			bodyJSON, err := ioutil.ReadAll(r.Body)
			if err != nil {
				return
			}

			var versions []struct {
				TagName string `json:"tag_name"`
				HtmlUrl string `json:"html_url"`
				Assets  []struct {
					BrowserDownloadUrl string `json:"browser_download_url"`
				} `json:"assets"`
			}

			//fmt.Println("body_json", len(bodyJSON))

			if err := json.Unmarshal(bodyJSON, &versions); err != nil {
				log.Println(err, string(bodyJSON))
				return
			}

			//fmt.Println(versions)

			if versions != nil && len(versions) > 0 {

				latestVersion := versions[0].HtmlUrl

				fmt.Println(latestVersion, version)

				if latestVersion != version {
					_latestVersionUrl = latestVersion
					_openUpdatePopup = 1

					for _, e := range versions[0].Assets {
						if strings.HasSuffix(e.BrowserDownloadUrl, runtime.GOOS+".zip") {
							_latestDownload = e.BrowserDownloadUrl
						}
					}
				}
			}
		}()
	}

	if _openUpdatePopup == 1 {
		_openUpdatePopup = 2
		imgui.OpenPopup("Update")
	}

	//imgui.SetNextWindowSize(imgui.Vec2{X: float32(400), Y: float32(130)})
	if imgui.BeginPopupModalV("Update", nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings) {
		imgui.Spacing()
		imgui.Spacing()
		imgui.Text("New Release " + _latestVersionUrl[strings.LastIndex(_latestVersionUrl, "/")+1:] + " is available:")
		imgui.Spacing()
		imgui.Spacing()
		imgui.Separator()
		imgui.Text("")
		if _updateErr != nil {
			imgui.Text("Error: " + _updateErr.Error())
		} else {
			ImguiHyperLink(_latestVersionUrl, _latestVersionUrl)
		}
		imgui.Text("")
		imgui.Text("")
		imgui.Text("")

		imgui.SetCursorPos(imgui.WindowSize().Minus(imgui.Vec2{X: float32(40), Y: float32(38)}))
		imgui.Separator()

		if _latestDownload != "" {

			imgui.SetCursorPos(imgui.WindowSize().Minus(imgui.Vec2{X: float32(130), Y: float32(28)}))
			if imgui.Button("Download") {
				_updateErr = update(version)
				if _updateErr != nil {
					log.Println(_updateErr)
					_openUpdatePopup = 0
				}
			}
			imgui.SameLine()
			if imgui.Button("Cancel") {
				imgui.CloseCurrentPopup()
			}
		} else {
			imgui.SetCursorPos(imgui.WindowSize().Minus(imgui.Vec2{X: float32(40), Y: float32(28)}))
			if imgui.Button("OK") {
				imgui.CloseCurrentPopup()
			}
		}
		imgui.EndPopup()
	}
}
