package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "ytpgui",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Fully transparent: the OS translucency effects below show through,
		// and the CSS paints a semi-transparent dark gray on top.
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WindowIsTranslucent: true,
			// Acrylic is the classic frosted-glass blur. windows.Mica is the
			// subtler Windows 11 desktop-tinted alternative if you prefer it.
			BackdropType: windows.Acrylic,
		},
		Mac: &mac.Options{
			WindowIsTranslucent: true,
			// Force dark appearance so the native titlebar/controls match the theme.
			Appearance: mac.NSAppearanceNameDarkAqua,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
