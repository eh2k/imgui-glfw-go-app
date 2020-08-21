package app

import (
	"github.com/eh2k/imgui-glfw-go-app/imgui-go"
	//"./imgui-go"
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/google/uuid"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"
)

func glRenderImgui(displaySize imgui.Vec2, framebufferSize imgui.Vec2, drawData imgui.DrawData) {
	// Avoid rendering when minimized, scale coordinates for retina displays (screen coordinates != framebuffer coordinates)
	displayWidth, displayHeight := displaySize.X, displaySize.Y
	fbWidth, fbHeight := framebufferSize.X, framebufferSize.Y
	if (fbWidth <= 0) || (fbHeight <= 0) {
		return
	}
	drawData.ScaleClipRects(imgui.Vec2{
		X: fbWidth / displayWidth,
		Y: fbHeight / displayHeight,
	})

	// Setup render state: alpha-blending enabled, no face culling, no depth testing, scissor enabled, vertex/texcoord/color pointers, polygon fill.
	var lastTexture int32
	gl.GetIntegerv(gl.TEXTURE_BINDING_2D, &lastTexture)
	var lastPolygonMode [2]int32
	gl.GetIntegerv(gl.POLYGON_MODE, &lastPolygonMode[0])
	var lastViewport [4]int32
	gl.GetIntegerv(gl.VIEWPORT, &lastViewport[0])
	var lastScissorBox [4]int32
	gl.GetIntegerv(gl.SCISSOR_BOX, &lastScissorBox[0])
	gl.PushAttrib(gl.ENABLE_BIT | gl.COLOR_BUFFER_BIT | gl.TRANSFORM_BIT)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.CULL_FACE)
	gl.Disable(gl.DEPTH_TEST)
	gl.Disable(gl.LIGHTING)
	gl.Disable(gl.COLOR_MATERIAL)
	gl.Enable(gl.SCISSOR_TEST)
	gl.EnableClientState(gl.VERTEX_ARRAY)
	gl.EnableClientState(gl.TEXTURE_COORD_ARRAY)
	gl.EnableClientState(gl.COLOR_ARRAY)
	gl.Enable(gl.TEXTURE_2D)
	gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL)

	// You may want this if using this code in an OpenGL 3+ context where shaders may be bound
	gl.UseProgram(0)

	// Setup viewport, orthographic projection matrix
	// Our visible imgui space lies from draw_data->DisplayPos (top left) to draw_data->DisplayPos+data_data->DisplaySize (bottom right).
	// DisplayMin is typically (0,0) for single viewport apps.
	gl.Viewport(0, 0, int32(fbWidth), int32(fbHeight))
	gl.MatrixMode(gl.PROJECTION)
	gl.PushMatrix()
	gl.LoadIdentity()
	gl.Ortho(0, float64(displayWidth), float64(displayHeight), 0, -1, 1)
	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()
	gl.LoadIdentity()

	vertexSize, vertexOffsetPos, vertexOffsetUv, vertexOffsetCol := imgui.VertexBufferLayout()
	indexSize := imgui.IndexBufferLayout()

	drawType := gl.UNSIGNED_SHORT
	if indexSize == 4 {
		drawType = gl.UNSIGNED_INT
	}

	// Render command lists
	for _, commandList := range drawData.CommandLists() {
		vertexBuffer, _ := commandList.VertexBuffer()
		indexBuffer, _ := commandList.IndexBuffer()
		indexBufferOffset := uintptr(indexBuffer)

		gl.VertexPointer(2, gl.FLOAT, int32(vertexSize), unsafe.Pointer(uintptr(vertexBuffer)+uintptr(vertexOffsetPos)))
		gl.TexCoordPointer(2, gl.FLOAT, int32(vertexSize), unsafe.Pointer(uintptr(vertexBuffer)+uintptr(vertexOffsetUv)))
		gl.ColorPointer(4, gl.UNSIGNED_BYTE, int32(vertexSize), unsafe.Pointer(uintptr(vertexBuffer)+uintptr(vertexOffsetCol)))

		for _, command := range commandList.Commands() {
			if command.HasUserCallback() {
				command.CallUserCallback(commandList)
			} else {
				clipRect := command.ClipRect()
				gl.Scissor(int32(clipRect.X), int32(fbHeight)-int32(clipRect.W), int32(clipRect.Z-clipRect.X), int32(clipRect.W-clipRect.Y))
				gl.BindTexture(gl.TEXTURE_2D, uint32(command.TextureID()))
				gl.DrawElements(gl.TRIANGLES, int32(command.ElementCount()), uint32(drawType), unsafe.Pointer(indexBufferOffset))
			}

			indexBufferOffset += uintptr(command.ElementCount() * indexSize)
		}
	}

	err := gl.GetError()
	if err != 0 {
		log.Panic(err)
	}
	// Restore modified state
	gl.DisableClientState(gl.COLOR_ARRAY)
	gl.DisableClientState(gl.TEXTURE_COORD_ARRAY)
	gl.DisableClientState(gl.VERTEX_ARRAY)
	gl.BindTexture(gl.TEXTURE_2D, uint32(lastTexture))
	gl.MatrixMode(gl.MODELVIEW)
	gl.PopMatrix()
	gl.MatrixMode(gl.PROJECTION)
	gl.PopMatrix()
	gl.PopAttrib()
	gl.PolygonMode(gl.FRONT, uint32(lastPolygonMode[0]))
	gl.PolygonMode(gl.BACK, uint32(lastPolygonMode[1]))
	gl.Viewport(lastViewport[0], lastViewport[1], lastViewport[2], lastViewport[3])
	gl.Scissor(lastScissorBox[0], lastScissorBox[1], lastScissorBox[2], lastScissorBox[3])
}

func createFontsTexture(io imgui.IO) uint32 {
	// Build texture atlas
	image := io.Fonts().TextureDataRGBA32()

	// Upload texture to graphics system
	var lastTexture int32
	var fontTexture uint32
	gl.GetIntegerv(gl.TEXTURE_BINDING_2D, &lastTexture)
	gl.GenTextures(1, &fontTexture)
	gl.BindTexture(gl.TEXTURE_2D, fontTexture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(image.Width), int32(image.Height), 0, gl.RGBA, gl.UNSIGNED_BYTE, image.Pixels)

	// Store our identifier
	io.Fonts().SetTextureID(imgui.TextureID(fontTexture))

	// Restore state
	gl.BindTexture(gl.TEXTURE_2D, uint32(lastTexture))

	return fontTexture
}

func destroyFontsTexture(fontTexture uint32) {
	if fontTexture != 0 {
		gl.DeleteTextures(1, &fontTexture)
		imgui.CurrentIO().Fonts().SetTextureID(0)
		fontTexture = 0
	}
}

var (
	textureMap = make(map[string]uint32)
)

// GetTexture ... any = file or []byte
func GetTexture(any interface{}) imgui.TextureID {

	id := ""
	var f io.Reader

	byteArr, ok := any.([]byte)
	if ok {
		id = fmt.Sprintf("%d", &byteArr[0])
		if textureMap[id] != 0 {
			return imgui.TextureID(textureMap[id])
		}

		f = bytes.NewReader(byteArr)
	} else {
		file, ok := any.(string)
		id = file
		if ok {
			if textureMap[id] != 0 {
				return imgui.TextureID(textureMap[file])
			}

			ff, err := os.Open(file)
			if err == nil {
				f = ff
			} else {
				log.Fatal(err)
			}
		}
	}

	png, err := png.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	img := imgui.RGBA32Image{Width: png.Bounds().Dx(), Height: png.Bounds().Dy()}

	rgba := image.NewRGBA(png.Bounds())

	for x := 0; x < img.Width; x++ {
		for y := 0; y < img.Height; y++ {
			pixel := png.At(x, y)
			rgba.Set(x, y, pixel)
		}
	}

	// Upload texture to graphics system
	var lastTexture int32
	var newTexture uint32
	gl.GetIntegerv(gl.TEXTURE_BINDING_2D, &lastTexture)
	gl.GenTextures(1, &newTexture)
	gl.BindTexture(gl.TEXTURE_2D, newTexture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(img.Width), int32(img.Height), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))

	// Restore state
	gl.BindTexture(gl.TEXTURE_2D, uint32(lastTexture))

	textureMap[id] = newTexture

	return imgui.TextureID(newTexture)
}

func InitMyImguiStyle() {

	//https://github.com/inkyblackness/hacked/blob/b8ec8e13df6f9dc4a416d4b375139e78d8120035/editor/Application.go

	imgui.CurrentStyle().ScaleAllSizes(1)

	style := imgui.CurrentStyle()
	style.SetColor(imgui.StyleColorText, imgui.Vec4{0.00, 0.00, 0.00, 1.00})
	style.SetColor(imgui.StyleColorTextDisabled, imgui.Vec4{0.60, 0.60, 0.60, 1.00})
	style.SetColor(imgui.StyleColorWindowBg, imgui.Vec4{0.94, 0.94, 0.94, 1.00})
	style.SetColor(imgui.StyleColorChildBg, imgui.Vec4{0.00, 0.00, 0.00, 0.00})
	style.SetColor(imgui.StyleColorPopupBg, imgui.Vec4{1.00, 1.00, 1.00, 0.98})
	style.SetColor(imgui.StyleColorBorder, imgui.Vec4{0.00, 0.00, 0.00, 0.30})
	style.SetColor(imgui.StyleColorBorderShadow, imgui.Vec4{0.00, 0.00, 0.00, 0.00})
	style.SetColor(imgui.StyleColorFrameBg, imgui.Vec4{1.00, 1.00, 1.00, 1.00})
	style.SetColor(imgui.StyleColorFrameBgHovered, imgui.Vec4{0.26, 0.59, 0.98, 0.40})
	style.SetColor(imgui.StyleColorFrameBgActive, imgui.Vec4{0.26, 0.59, 0.98, 0.67})
	style.SetColor(imgui.StyleColorTitleBg, imgui.Vec4{0.96, 0.96, 0.96, 1.00})
	style.SetColor(imgui.StyleColorTitleBgActive, imgui.Vec4{0.82, 0.82, 0.82, 1.00})
	style.SetColor(imgui.StyleColorTitleBgCollapsed, imgui.Vec4{1.00, 1.00, 1.00, 0.51})
	style.SetColor(imgui.StyleColorMenuBarBg, imgui.Vec4{0.86, 0.86, 0.86, 1.00})
	style.SetColor(imgui.StyleColorScrollbarBg, imgui.Vec4{0.98, 0.98, 0.98, 0.53})
	style.SetColor(imgui.StyleColorScrollbarGrab, imgui.Vec4{0.69, 0.69, 0.69, 0.80})
	style.SetColor(imgui.StyleColorScrollbarGrabHovered, imgui.Vec4{0.49, 0.49, 0.49, 0.80})
	style.SetColor(imgui.StyleColorScrollbarGrabActive, imgui.Vec4{0.49, 0.49, 0.49, 1.00})
	style.SetColor(imgui.StyleColorCheckMark, imgui.Vec4{0.26, 0.59, 0.98, 1.00})
	style.SetColor(imgui.StyleColorSliderGrab, imgui.Vec4{0.26, 0.59, 0.98, 0.78})
	style.SetColor(imgui.StyleColorSliderGrabActive, imgui.Vec4{0.46, 0.54, 0.80, 0.60})
	style.SetColor(imgui.StyleColorButton, imgui.Vec4{0.26, 0.59, 0.98, 0.40})
	style.SetColor(imgui.StyleColorButtonHovered, imgui.Vec4{0.26, 0.59, 0.98, 1.00})
	style.SetColor(imgui.StyleColorButtonActive, imgui.Vec4{0.06, 0.53, 0.98, 1.00})
	style.SetColor(imgui.StyleColorHeader, imgui.Vec4{0.26, 0.59, 0.98, 0.31})
	style.SetColor(imgui.StyleColorHeaderHovered, imgui.Vec4{0.26, 0.59, 0.98, 0.80})
	style.SetColor(imgui.StyleColorTabHovered, imgui.Vec4{0.26, 0.59, 0.98, 0.80})
	style.SetColor(imgui.StyleColorNavHighlight, imgui.Vec4{0.26, 0.59, 0.98, 0.80})
	style.SetColor(imgui.StyleColorHeaderActive, imgui.Vec4{0.26, 0.59, 0.98, 1.00})
	style.SetColor(imgui.StyleColorSeparator, imgui.Vec4{0.39, 0.39, 0.39, 0.62})
	style.SetColor(imgui.StyleColorSeparatorHovered, imgui.Vec4{0.14, 0.44, 0.80, 0.78})
	style.SetColor(imgui.StyleColorSeparatorActive, imgui.Vec4{0.14, 0.44, 0.80, 1.00})
	style.SetColor(imgui.StyleColorResizeGrip, imgui.Vec4{0.80, 0.80, 0.80, 0.56})
	style.SetColor(imgui.StyleColorResizeGripHovered, imgui.Vec4{0.26, 0.59, 0.98, 0.67})
	style.SetColor(imgui.StyleColorResizeGripActive, imgui.Vec4{0.26, 0.59, 0.98, 0.95})
	style.SetColor(imgui.StyleColorPlotLines, imgui.Vec4{0.39, 0.39, 0.39, 1.00})
	style.SetColor(imgui.StyleColorPlotLinesHovered, imgui.Vec4{1.00, 0.43, 0.35, 1.00})
	style.SetColor(imgui.StyleColorPlotHistogram, imgui.Vec4{0.90, 0.70, 0.00, 1.00})
	style.SetColor(imgui.StyleColorPlotHistogramHovered, imgui.Vec4{1.00, 0.45, 0.00, 1.00})
	style.SetColor(imgui.StyleColorTextSelectedBg, imgui.Vec4{0.26, 0.59, 0.98, 0.35})
	style.SetColor(imgui.StyleColorDragDropTarget, imgui.Vec4{0.26, 0.59, 0.98, 0.95})
	style.SetColor(imgui.StyleColorNavWindowingHighlight, imgui.Vec4{0.70, 0.70, 0.70, 0.70})
	//style.SetColor(imgui.StylTab,                    = ImLerp(style.SetColor(imgui.StylHeader,,       style.SetColor(imgui.StylTitleBgActive,, 0.90})
	//style.SetColor(imgui.StylTabActive,              = ImLerp(style.SetColor(imgui.StylHeaderActive,, style.SetColor(imgui.StylTitleBgActive,, 0.60})
	//style.SetColor(imgui.StylTabUnfocused,           = ImLerp(style.SetColor(imgui.StylTab,,          style.SetColor(imgui.StylTitleBg,, 0.80})
	//style.SetColor(imgui.StylTabUnfocusedActive,     = ImLerp(style.SetColor(imgui.StylTabActive,,    style.SetColor(imgui.StylTitleBg,, 0.40})
	//style.SetColor(imgui.StyleColorNavWindowingDimBg,      imgui.Vec4{0.20, 0.20, 0.20, 0.20})
	//style.SetColor(imgui.StyleColorModalWindowDimBg,       imgui.Vec4{0.20, 0.20, 0.20, 0.35})
}

func OpenUrl(url string) {
	cmd := ""
	if runtime.GOOS == "linux" {
		cmd = "dxg-open"
	} else if runtime.GOOS == "windows" {
		cmd = "explorer.exe"
	} else {
		cmd = "open"
	}

	err := exec.Command(cmd, url).Start()
	if err != nil {
		log.Println(err)
	}
}

func ImguiHyperLink(name string, url string) {

	// AddUnderLine := func(color imgui.PackedColor) {
	// 	min := imgui.CalcItemWidth()
	// 	max := imgui.GetItemRectMax()
	// 	min.y = max.y
	// 	imgui.WindowDrawList().AddLine(min, max, color, 1.0)
	// }

	imgui.PushStyleColor(imgui.StyleColorText, imgui.Vec4{X: 0.5, Y: 0.5, Z: 1, W: 1})
	imgui.Text(name)
	imgui.PopStyleColor()
	if imgui.IsItemHovered() {
		if imgui.IsMouseClicked(0) {
			OpenUrl(url)
		}
		//AddUnderLine(imgui.PackedColor(0x0000ff))
		imgui.SetTooltip("Open in browser... ")
	}
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

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return filenames, err
		}
	}
	return filenames, nil
}

func update(projectUrl string, oldVersion string) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	ver := _latestVersionUrl[strings.LastIndex(_latestVersionUrl, "/")+1:]
	url := fmt.Sprint(projectUrl,
		"/releases/download/",
		ver, "/",
		strings.ToLower(strings.Split(filepath.Base(exePath), ".")[0]),
		"-", runtime.GOOS, "-", ver, ".zip")

	zipFile := filepath.Join(os.TempDir(), uuid.New().String()+".zip")

	if _, err := os.Stat(zipFile); os.IsNotExist(err) {
		res, err := http.Get(url)
		if err != nil {
			return err
		}

		if err = os.MkdirAll(filepath.Join(filepath.Dir(exePath), ver), os.ModePerm); err != nil {
			return err
		}

		{
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
	}

	{
		_, err := Unzip(zipFile, filepath.Join(filepath.Dir(exePath), ver))

		if err != nil {
			return err
		}

		// err = os.Remove(zipFile)

		// if err != nil {
		// 	osdialog.ShowMessageBox(osdialog.Error, osdialog.Ok, err.Error())
		// 	return
		// }
	}

	{

		fmt.Println(exePath)
		newExe := filepath.Join(filepath.Dir(exePath), ver, filepath.Base(exePath))

		filepath.Walk(filepath.Join(filepath.Dir(exePath), ver), func (name string, info os.FileInfo, err error) error {
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

var _updateErr error = nil
var _latestVersionUrl = ""
var _openUpdatePopup = 0

func ShowAboutPopup(openPopup *bool, header string, version string, copyright string, projectUrl string) {

	if _latestVersionUrl == "" {
		_latestVersionUrl = version

		go func() {
			fmt.Println(projectUrl + "/releases/latest")
			r, err := http.Get(projectUrl + "/releases/latest")
			if err != nil {
				return
			}

			loc, _ := r.Request.Response.Location()
			latestVersion := loc.String()[strings.LastIndex(loc.String(), "/")+1:]

			fmt.Println(loc, latestVersion, loc.String())

			if latestVersion != version && strings.Contains(loc.String(), "tag") {
				_latestVersionUrl = loc.String()
				_openUpdatePopup = 1
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

		if _openUpdatePopup == 2 {

			imgui.SetCursorPos(imgui.WindowSize().Minus(imgui.Vec2{X: float32(130), Y: float32(28)}))
			if imgui.Button("Download") {
				_updateErr = update(projectUrl, version)
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

	if *openPopup {
		*openPopup = false
		imgui.OpenPopup("About")
	}

	imgui.SetNextWindowSize(imgui.Vec2{X: float32(400), Y: float32(230)})
	if imgui.BeginPopupModalV("About", nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings) {
		imgui.Spacing()
		imgui.Spacing()
		imgui.Text(header)
		imgui.SameLine()
		imgui.Text(version)
		imgui.Spacing()
		imgui.Spacing()
		imgui.Separator()
		imgui.Spacing()
		imgui.Spacing()
		imgui.Text(copyright)
		ImguiHyperLink(projectUrl, projectUrl)
		imgui.Spacing()
		imgui.Spacing()
		imgui.Spacing()
		imgui.Spacing()
		imgui.Separator()
		imgui.PushStyleColor(imgui.StyleColorText, imgui.Vec4{X: 0.5, Y: 0.5, Z: 0.5, W: 1})
		imgui.Text("OpenGL Version: " + gl.GoStr(gl.GetString(gl.VERSION)))
		imgui.Text("ImGui Version: " + imgui.Version())
		imgui.Text("GOOS/GOARCH: " + runtime.GOOS + "/" + runtime.GOARCH)
		//imgui.Separator()
		imgui.PopStyleColor()
		imgui.SetCursorPos(imgui.WindowSize().Minus(imgui.Vec2{X: float32(40), Y: float32(38)}))
		imgui.Separator()
		imgui.SetCursorPos(imgui.WindowSize().Minus(imgui.Vec2{X: float32(40), Y: float32(28)}))
		if imgui.Button("OK") {
			imgui.CloseCurrentPopup()
		}
		imgui.EndPopup()
	}
}

func ImguiToolbarsBegin() bool {
	imgui.SetNextWindowPos(imgui.Vec2{X: -4, Y: -4})
	if imgui.BeginV("toolbars", nil,
		imgui.WindowFlagsNoScrollbar|imgui.WindowFlagsNoTitleBar|imgui.WindowFlagsNoResize|imgui.WindowFlagsNoBackground|imgui.WindowFlagsNoSavedSettings) {
		imgui.PushStyleColor(imgui.StyleColorText, imgui.Vec4{X: 1, Y: 1, Z: 1, W: 0.8})
		imgui.PushStyleVarVec2(imgui.StyleVarItemSpacing, imgui.Vec2{X: 4, Y: 4})
		return true
	} else {
		return false
	}
}
func ImguiToolbarsEnd() {
	imgui.PopStyleVar()
	imgui.PopStyleColor()
	imgui.End()
}
func ImguiToolbar(header string, width float32, imguiContent func()) {
	//imgui.SetCursorPos(imgui.CursorPos().Minus(imgui.Vec2{X: 4, Y: 0}))
	if imgui.BeginChildV("toolbar_"+header, imgui.Vec2{X: width, Y: 48}, false, 0) {
		imgui.SetCursorPos(imgui.Vec2{X: 4, Y: 4})
		imgui.Text(header)
		imgui.SetCursorPos(imgui.Vec2{X: 4, Y: 20})
		imgui.PushItemWidth(width - 8)
		imgui.WindowDrawList().AddRectFilled(imgui.WindowPos(), imgui.WindowPos().Plus(imgui.WindowSize()), imgui.PackedColor(0x22ffffff))
		imguiContent()
	}

	imgui.EndChild()

	imgui.SameLine()
}

var (
	imguic      *imgui.Context
	fontTexture uint32
	mainWindow  *glfw.Window
)

func NewAppWindow(width int, height int) *glfw.Window {

	if mainWindow != nil {
		log.Fatal(errors.New("only one window supported"))
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	err = glfw.Init()
	if err != nil {
		log.Fatal(err)
	}

	window, err := glfw.CreateWindow(width, height, strings.Title(strings.Split(filepath.Base(exePath), ".")[0]), nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	if runtime.GOOS == "windows" {
		SetWindowIcon(window)
	}

	window.MakeContextCurrent()
	glfw.SwapInterval(1)

	if err := gl.Init(); err != nil {
		log.Fatalln("failed to initialize glfw:", err)
	}

	imguic = imgui.CreateContext(nil)
	io := imgui.CurrentIO()

	io.SetIniFilename("")

	io.Fonts().TextureDataAlpha8()
	imgui.StyleColorsClassic()

	fontTexture = createFontsTexture(io)

	mainWindow = window
	return window
}

func Dispose() {
	destroyFontsTexture(fontTexture)
	imguic.Destroy()
	defer glfw.Terminate()
}

func Run(frameContent func(displaySize imgui.Vec2)) {
	for !mainWindow.ShouldClose() {
		glfw.PollEvents()
		width, height := mainWindow.GetSize()

		imguiRender(width, height, frameContent)

		mainWindow.SwapBuffers()
	}
}

func imguiRender(width int, height int, frameContent func(displaySize imgui.Vec2)) {

	io := imgui.CurrentIO()

	displaySize := imgui.Vec2{X: float32(width), Y: float32(height)}
	io.SetDisplaySize(displaySize)

	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT | gl.STENCIL_BUFFER_BIT)

	imgui.NewFrame()

	frameContent(imgui.Vec2{X: float32(width), Y: float32(height)})

	imgui.Render()
	glRenderImgui(displaySize, displaySize, imgui.RenderedDrawData())
}
