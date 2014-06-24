/**
 * Copyright (c) 2011 ~ 2014 Deepin, Inc.
 *               2013 ~ 2014 jouyouyun
 *
 * Author:      jouyouyun <jouyouwen717@gmail.com>
 * Maintainer:  jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, see <http://www.gnu.org/licenses/>.
 **/

package themes

import (
	dutils "pkg.linuxdeepin.com/lib/utils"
	"path"
)

const (
	THUMB_CACHE_DIR = "autogen"
)

var (
	PERSON_SYS_THUMB_PATH   = path.Join(PERSON_SYS_BASE_PATH, "thumbnail")
	PERSON_LOCAL_THUMB_PATH = path.Join(homeDir, PERSON_LOCAL_BASE_PATH, "thumbnail")
)

func getCachePath(t, src string) string {
	src = dutils.URIToPath(src)
	bgDir := path.Join(PERSON_LOCAL_THUMB_PATH, THUMB_CACHE_DIR)
	md5Str, _ := getStrMd5(t + src)
	filename := path.Join(bgDir, md5Str+".png")
	if dutils.IsFileExist(filename) {
		return filename
	}

	filename = path.Join(PERSON_SYS_THUMB_PATH, THUMB_CACHE_DIR, md5Str+".png")
	if dutils.IsFileExist(filename) {
		return filename
	}

	return ""
}

func getDThemeThumb(name string) string {
	if name == "Custom" {
		if t, ok := GetManager().themeObjMap[name]; ok {
			return GetManager().GetThumbnail("background", t.Background)
		}
		return ""
	}

	list := getDThemeList()
	for _, l := range list {
		if name == l.Name {
			thumb := path.Join(l.Path, "thumbnail.png")
			if dutils.IsFileExist(thumb) {
				return thumb
			}
			break
		}
	}

	return ""
}

func getGtkThumb(name string) string {
	list := getGtkThemeList()

	for _, l := range list {
		if name == l.Name {
			dest := ""
			if l.T == THEME_TYPE_SYSTEM {
				dest = path.Join(PERSON_SYS_THUMB_PATH,
					"WindowThemes", name+"-thumbnail.png")
			} else {
				dest = path.Join(PERSON_LOCAL_THUMB_PATH,
					"WindowThemes", name+"-thumbnail.png")
			}
			if dutils.IsFileExist(dest) {
				return dest
			}

			dest = getCachePath("--gtk", l.Path)
			if len(dest) > 0 {
				return dest
			}
			break
		}
	}

	return ""
}

func getIconThumb(name string) string {
	list := getIconThemeList()

	for _, l := range list {
		if name == l.Name {
			dest := ""
			if l.T == THEME_TYPE_SYSTEM {
				dest = path.Join(PERSON_SYS_THUMB_PATH,
					"IconThemes", name+"-thumbnail.png")
			} else {
				dest = path.Join(PERSON_LOCAL_THUMB_PATH,
					"IconThemes", name+"-thumbnail.png")
			}
			if dutils.IsFileExist(dest) {
				return dest
			}

			dest = getCachePath("--icon", l.Path)
			if len(dest) > 0 {
				return dest
			}
			break
		}
	}

	return ""
}

func getCursorThumb(name string) string {
	list := getCursorThemeList()

	for _, l := range list {
		if name == l.Name {
			dest := ""
			if l.T == THEME_TYPE_SYSTEM {
				dest = path.Join(PERSON_SYS_THUMB_PATH,
					"CursorThemes", name+"-thumbnail.png")
			} else {
				dest = path.Join(PERSON_LOCAL_THUMB_PATH,
					"CursorThemes", name+"-thumbnail.png")
			}
			if dutils.IsFileExist(dest) {
				return dest
			}

			dest = getCachePath("--cursor", l.Path)
			if len(dest) > 0 {
				return dest
			}
			break
		}
	}

	return ""
}

func getBgThumb(bg string) string {
	dest := getCachePath("--background", bg)
	if len(dest) > 0 {
		return dest
	}

	genThemeThumb()
	bg = dutils.PathToURI(bg, dutils.SCHEME_FILE)
	return bg
}