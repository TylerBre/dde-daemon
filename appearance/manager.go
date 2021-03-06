/*
 * Copyright (C) 2014 ~ 2017 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package appearance

import (
	"dbus/com/deepin/daemon/accounts"
	"dbus/com/deepin/wm"
	"encoding/json"
	"fmt"
	"gir/gio-2.0"
	"io/ioutil"
	"os/user"
	"pkg.deepin.io/dde/api/theme_thumb"
	"pkg.deepin.io/dde/daemon/appearance/background"
	"pkg.deepin.io/dde/daemon/appearance/fonts"
	"pkg.deepin.io/dde/daemon/appearance/subthemes"
	ddbus "pkg.deepin.io/dde/daemon/dbus"
	"pkg.deepin.io/lib/dbus/property"
	"pkg.deepin.io/lib/fsnotify"
	dutils "pkg.deepin.io/lib/utils"
	"strconv"
	"time"
)

// The supported types
const (
	TypeGtkTheme          = "gtk"
	TypeIconTheme         = "icon"
	TypeCursorTheme       = "cursor"
	TypeBackground        = "background"
	TypeGreeterBackground = "greeterbackground"
	TypeStandardFont      = "standardfont"
	TypeMonospaceFont     = "monospacefont"
	TypeFontSize          = "fontsize"
)

const (
	wrapBgSchema    = "com.deepin.wrap.gnome.desktop.background"
	gnomeBgSchema   = "org.gnome.desktop.background"
	gsKeyBackground = "picture-uri"

	appearanceSchema    = "com.deepin.dde.appearance"
	gsKeyGtkTheme       = "gtk-theme"
	gsKeyIconTheme      = "icon-theme"
	gsKeyCursorTheme    = "cursor-theme"
	gsKeyFontStandard   = "font-standard"
	gsKeyFontMonospace  = "font-monospace"
	gsKeyFontSize       = "font-size"
	gsKeyBackgroundURIs = "background-uris"

	defaultStandardFont   = "Noto Sans"
	defaultMonospaceFont  = "Noto Mono"
	defaultFontConfigFile = "/usr/share/deepin-default-settings/fontconfig.json"
)

// Manager shows current themes and fonts settings, emit 'Changed' signal if modified
// if themes list changed will emit 'Refreshed' signal
type Manager struct {
	GtkTheme      *property.GSettingsStringProperty
	IconTheme     *property.GSettingsStringProperty
	CursorTheme   *property.GSettingsStringProperty
	Background    *property.GSettingsStringProperty
	StandardFont  *property.GSettingsStringProperty
	MonospaceFont *property.GSettingsStringProperty

	FontSize *property.GSettingsFloatProperty

	// Signals:
	// Theme setting changed
	Changed func(_type string, name string)
	// Theme list refreshed
	Refreshed func(_type string)

	userObj *accounts.User

	setting        *gio.Settings
	wrapBgSetting  *gio.Settings
	gnomeBgSetting *gio.Settings

	defaultFontConfig DefaultFontConfig

	watcher    *fsnotify.Watcher
	endWatcher chan struct{}

	wm *wm.Wm
}

// NewManager will create a 'Manager' object
func NewManager() *Manager {
	var m = new(Manager)
	m.setting = gio.NewSettings(appearanceSchema)
	m.wrapBgSetting = gio.NewSettings(wrapBgSchema)

	m.GtkTheme = property.NewGSettingsStringProperty(
		m, "GtkTheme",
		m.setting, gsKeyGtkTheme)
	m.IconTheme = property.NewGSettingsStringProperty(
		m, "IconTheme",
		m.setting, gsKeyIconTheme)
	m.CursorTheme = property.NewGSettingsStringProperty(
		m, "CursorTheme",
		m.setting, gsKeyCursorTheme)
	m.StandardFont = property.NewGSettingsStringProperty(
		m, "StandardFont",
		m.setting, gsKeyFontStandard)
	m.MonospaceFont = property.NewGSettingsStringProperty(
		m, "MonospaceFont",
		m.setting, gsKeyFontMonospace)
	m.Background = property.NewGSettingsStringProperty(
		m, "Background",
		m.wrapBgSetting, gsKeyBackground)

	m.FontSize = property.NewGSettingsFloatProperty(
		m, "FontSize",
		m.setting, gsKeyFontSize)

	m.gnomeBgSetting, _ = dutils.CheckAndNewGSettings(gnomeBgSchema)

	var err error
	m.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		logger.Warning("New file watcher failed:", err)
	} else {
		m.endWatcher = make(chan struct{})
	}

	return m
}

func (m *Manager) destroy() {
	if m.setting != nil {
		m.setting.Unref()
		m.setting = nil
	}

	if m.wrapBgSetting != nil {
		m.wrapBgSetting.Unref()
		m.wrapBgSetting = nil
	}

	if m.gnomeBgSetting != nil {
		m.gnomeBgSetting.Unref()
		m.gnomeBgSetting = nil
	}

	if m.userObj != nil {
		ddbus.DestroyUser(m.userObj)
		m.userObj = nil
	}

	if m.watcher != nil {
		close(m.endWatcher)
		m.watcher.Close()
		m.watcher = nil
	}

	if m.wm != nil {
		wm.DestroyWm(m.wm)
		m.wm = nil
	}

	m.endCursorChangedHandler()
}

// resetFonts reset StandardFont and MonospaceFont
func (m *Manager) resetFonts() {
	defaultStandardFont, defaultMonospaceFont := m.getDefaultFonts()
	logger.Debugf("getDefaultFonts standard: %q, mono: %q",
		defaultStandardFont, defaultMonospaceFont)
	if defaultStandardFont != m.StandardFont.Get() {
		m.StandardFont.Set(defaultStandardFont)
	}

	if defaultMonospaceFont != m.MonospaceFont.Get() {
		m.MonospaceFont.Set(defaultMonospaceFont)
	}

	err := fonts.SetFamily(defaultStandardFont, defaultMonospaceFont,
		m.FontSize.Get())
	if err != nil {
		logger.Debug("resetFonts fonts.SetFamily failed", err)
		return
	}
	m.checkFontConfVersion()
}

func (m *Manager) init() {
	theme_thumb.Init(doGetScaleFactor())

	// Init theme list
	time.AfterFunc(time.Second*10, func() {
		if !dutils.IsFileExist(fonts.DeepinFontConfig) {
			m.resetFonts()
		} else {
			m.correctFontName()
		}

		subthemes.ListGtkTheme()
		subthemes.ListIconTheme()
		subthemes.ListCursorTheme()
		background.ListBackground()
		fonts.GetFamilyTable()

		// must be called after init finished
		go m.handleThemeChanged()
		m.listenGSettingChanged()

		setDQtTheme(dQtFile, dQtSectionTheme,
			[]string{
				dQtKeyIcon,
				dQtKeyFont,
				dQtKeyMonoFont,
				dQtKeyFontSize},
			[]string{
				m.IconTheme.Get(),
				m.StandardFont.Get(),
				m.MonospaceFont.Get(),
				strconv.FormatFloat(m.FontSize.Get(), 'f', 1, 64)})
		err := saveDQtTheme(dQtFile)
		if err != nil {
			logger.Warning("Failed to save qt theme:", err)
			return
		}
	})

	cur, err := user.Current()
	if err != nil {
		logger.Warning("Get current user info failed:", err)
	} else {
		m.userObj, err = ddbus.NewUserByUid(cur.Uid)
		if err != nil {
			logger.Warning("New user object failed:", cur.Name, err)
			m.userObj = nil
		}
	}

	m.wm, err = wm.NewWm("com.deepin.wm", "/com/deepin/wm")
	if err != nil {
		logger.Warning("new wm failed:", err)
	}

	err = m.loadDefaultFontConfig(defaultFontConfigFile)
	if err != nil {
		logger.Warning("load default font config failed:", err)
	} else {
		logger.Debugf("load default font config ok %#v", m.defaultFontConfig)
	}

	m.initBackground()
	m.doSetGtkTheme(m.GtkTheme.Get())
	m.doSetIconTheme(m.IconTheme.Get())
	m.doSetCursorTheme(m.CursorTheme.Get())
}

func (m *Manager) correctFontName() {
	defaultStandardFont, defaultMonospaceFont := m.getDefaultFonts()

	var changed bool = false
	table := fonts.GetFamilyTable()
	stand := table.GetFamily(m.StandardFont.Get())
	if stand != nil {
		// for virtual font
		if stand.Id != m.StandardFont.Get() {
			changed = true
			m.StandardFont.Set(stand.Id)
		}
	} else {
		changed = true
		m.StandardFont.Set(defaultStandardFont)
	}

	mono := table.GetFamily(m.MonospaceFont.Get())
	if mono != nil {
		if mono.Id != m.MonospaceFont.Get() {
			changed = true
			m.MonospaceFont.Set(mono.Id)
		}
	} else {
		changed = true
		m.MonospaceFont.Set(defaultMonospaceFont)
	}

	if !changed && m.checkFontConfVersion() {
		return
	}

	err := fonts.SetFamily(m.StandardFont.Get(), m.MonospaceFont.Get(),
		m.FontSize.Get())
	if err != nil {
		logger.Debug("[correctFontName]-----------set font failed:", err)
		return
	}
}

func (m *Manager) doSetGtkTheme(value string) error {
	if !subthemes.IsGtkTheme(value) {
		return fmt.Errorf("Invalid gtk theme '%v'", value)
	}

	return subthemes.SetGtkTheme(value)
}

func (m *Manager) doSetIconTheme(value string) error {
	if !subthemes.IsIconTheme(value) {
		return fmt.Errorf("Invalid icon theme '%v'", value)
	}

	err := subthemes.SetIconTheme(value)
	if err != nil {
		return err
	}

	return m.writeDQtTheme(dQtKeyIcon, value)
}

func (m *Manager) doSetCursorTheme(value string) error {
	if !subthemes.IsCursorTheme(value) {
		return fmt.Errorf("Invalid cursor theme '%v'", value)
	}

	return subthemes.SetCursorTheme(value)
}

func (m *Manager) doSetBackground(value string) (string, error) {
	logger.Debugf("call doSetBackground %q", value)
	if !background.IsBackgroundFile(value) {
		return "", fmt.Errorf("Invalid background file '%v'", value)
	}

	uri, err := background.ListBackground().EnsureExists(value)
	if err != nil {
		logger.Debugf("[doSetBackground] set '%s' failed: %v", value, uri, err)
		return "", err
	}

	if m.wm != nil && ddbus.IsSessionBusActivated(m.wm.DestName) {
		m.wm.ChangeCurrentWorkspaceBackground(uri)
	}

	if m.userObj != nil {
		m.userObj.SetBackgroundFile(uri)
	}
	return uri, nil
}

func (m *Manager) doSetGreeterBackground(value string) error {
	if m.userObj == nil {
		return fmt.Errorf("Create user object failed")
	}

	return m.userObj.SetGreeterBackground(value)
}

func (m *Manager) doSetStandardFont(value string) error {
	if !fonts.IsFontFamily(value) {
		return fmt.Errorf("Invalid font family '%v'", value)
	}

	err := fonts.SetFamily(value, m.MonospaceFont.Get(), m.FontSize.Get())
	if err != nil {
		return err
	}
	return m.writeDQtTheme(dQtKeyFont, value)
}

func (m *Manager) doSetMonnospaceFont(value string) error {
	if !fonts.IsFontFamily(value) {
		return fmt.Errorf("Invalid font family '%v'", value)
	}

	err := fonts.SetFamily(m.StandardFont.Get(), value, m.FontSize.Get())
	if err != nil {
		return err
	}
	return m.writeDQtTheme(dQtKeyMonoFont, value)
}

func (m *Manager) doSetFontSize(size float64) error {
	if !fonts.IsFontSizeValid(size) {
		logger.Debug("[doSetFontSize] invalid size:", size)
		return fmt.Errorf("Invalid font size '%v'", size)
	}

	err := fonts.SetFamily(m.StandardFont.Get(), m.MonospaceFont.Get(), size)
	if err != nil {
		return err
	}

	return m.writeDQtTheme(dQtKeyFontSize, strconv.FormatFloat(size, 'f', 1, 64))
}

func (*Manager) doShow(ifc interface{}) (string, error) {
	if ifc == nil {
		return "", fmt.Errorf("Not found target")
	}
	content, err := json.Marshal(ifc)
	return string(content), err
}

func (m *Manager) loadDefaultFontConfig(filename string) error {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	var defaultFontConfig DefaultFontConfig
	if err := json.Unmarshal(contents, &defaultFontConfig); err != nil {
		return err
	}

	m.defaultFontConfig = defaultFontConfig
	return nil
}

func (m *Manager) getDefaultFonts() (standard string, monospace string) {
	cfg := m.defaultFontConfig
	if cfg == nil {
		return defaultStandardFont, defaultMonospaceFont
	}
	return cfg.Get()
}

func (m *Manager) writeDQtTheme(key, value string) error {
	setDQtTheme(dQtFile, dQtSectionTheme,
		[]string{key}, []string{value})
	return saveDQtTheme(dQtFile)
}
