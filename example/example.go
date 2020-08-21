//go build -x -ldflags=all='-H windowsgui -s -w' -o ./bin/uwedit.exe

package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"github.com/eh2k/osdialog-go"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/eh2k/imgui-glfw-go-app"
	"github.com/eh2k/imgui-glfw-go-app/imgui-go"
	//".."
	//"../imgui-go"
)

func init() {
	// This is needed to arrange that main() runs on main thread.
	// See documentation for functions that are only allowed to be called from the main thread.
	runtime.LockOSThread()
}

var (
	loadProgress = float32(0)
	showDemoWindow = true
)

func loop(displaySize imgui.Vec2) {

	var openFileDialog = false
	var saveFileDialog = false
	var showAboutWindow = false

	imgui.ShowUserGuide()

	if app.ImguiToolbarsBegin() {

		app.ImguiToolbar("File", 130, func() {

			if imgui.Button("Open..") {
				openFileDialog = true
			}

			imgui.SameLine()

			if imgui.Button("Save..") {
				saveFileDialog = true
			}
		})

		app.ImguiToolbar("About", 54, func() {

			if imgui.Button("Info") {
				showAboutWindow = true
			}
		})

		if !showDemoWindow {
		app.ImguiToolbar("Imgui", 54, func() {

			if imgui.Button("Demo") {
				showDemoWindow = true
			}
		})
	}

		app.ImguiToolbarsEnd()
	}

	app.ShowAboutPopup(&showAboutWindow, "imgui-glfw-go-app", "1.0", "Copyright (C) 2020 by E.Heidt", "https://github.com/eh2k/imgui-glfw-go-app")

	imgui.SetNextWindowPos(imgui.Vec2{X: displaySize.X / 2 - 150.0, Y: displaySize.Y /2 - 20.0})
	if imgui.BeginPopupModalV("Upload", nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings|imgui.WindowFlagsNoTitleBar) {
		imgui.Text("uploading...")
		imgui.ProgressBarV(loadProgress, imgui.Vec2{X: 300, Y: 22}, "")
		if loadProgress >= 1 {
			imgui.CloseCurrentPopup()
		}

		imgui.EndPopup()
	}

	if showDemoWindow {
		imgui.SetNextWindowPosV(imgui.Vec2{X: 150, Y: 20}, imgui.ConditionFirstUseEver, imgui.Vec2{})
		imgui.ShowDemoWindow(&showDemoWindow)
	}

	if openFileDialog {
		openFileDialog = false
		filename, err := osdialog.ShowOpenFileDialog(".", "", "Files:*")
		if err == nil 	{
			imgui.OpenPopup("Upload")
	
			go func() {
	
				loadProgress = 0
				time.Sleep(0)
				for j := 0; j < 10; j++ {
					time.Sleep(50000000)
					loadProgress += 0.1
					fmt.Println(filename, loadProgress)
				}
				loadProgress = 1
			}()
		}
	}

	if saveFileDialog {
		saveFileDialog = false
		filename, err := osdialog.ShowSaveFileDialog(".", "", "Files:*")
		if err == nil {
			osdialog.ShowMessageBox(osdialog.Info, osdialog.Ok, filename)
		}
	}
}

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	exePath, err := os.Executable()
	if err != nil {
		log.Println(err)
	}

	err3 := os.Chdir(filepath.Dir(exePath))
	if err3 != nil {
		log.Println(err3)
	}

	window := app.NewAppWindow(1024, 768)
	defer app.Dispose()

	app.InitMyImguiStyle()

	{
		io := imgui.CurrentIO()

		log.Println("OK")

		window.SetScrollCallback(func(w *glfw.Window, xoff float64, yoff float64) {
			if io.WantCaptureMouse() {
				io.AddMouseWheelDelta(float32(xoff), float32(yoff))
			} else {
				//context.OnMouseWheel(float32(yoff))
			}
		})

		window.SetMouseButtonCallback(func(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {

			if io.WantCaptureMouse() {
				io.SetMouseButtonDown(int(button), action == 1)
			} else {
				//x, y := w.GetCursorPos()
				//context.OnMouseMove(w, x, y)
			}
		})

		window.SetCursorPosCallback(func(w *glfw.Window, x float64, y float64) {

			if w.GetAttrib(glfw.Focused) != 0 {
				io.SetMousePosition(imgui.Vec2{X: float32(x), Y: float32(y)})
			} else {
				io.SetMousePosition(imgui.Vec2{X: -math.MaxFloat32, Y: -math.MaxFloat32})
			}

			if !io.WantCaptureMouse() {
				//context.OnMouseMove(w, x, y)
			}
		})

		io.KeyMap(imgui.KeyBackspace, int(glfw.KeyBackspace))

		window.SetCharCallback(func(window *glfw.Window, char rune) {
			io.AddInputCharacters(string(char))
		})

		window.SetKeyCallback(func(window *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
			if action == glfw.Press {
				io.KeyPress(int(key))
			}
			if action == glfw.Release {
				io.KeyRelease(int(key))
			}

			io.KeyCtrl(int(glfw.KeyLeftControl), int(glfw.KeyRightControl))
			io.KeyShift(int(glfw.KeyLeftShift), int(glfw.KeyRightShift))
			io.KeyAlt(int(glfw.KeyLeftAlt), int(glfw.KeyRightAlt))
			io.KeySuper(int(glfw.KeyLeftSuper), int(glfw.KeyRightSuper))
		})
	}

	app.Run(loop);

	fmt.Println("END")
}
