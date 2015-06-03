/**
 * Copyright (c) 2011 ~ 2015 Deepin, Inc.
 *               2013 ~ 2015 jouyouyun
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

package timedate

import (
	"pkg.linuxdeepin.com/lib/dbus"
	"pkg.linuxdeepin.com/lib/log"
)

var (
	_manager *Manager

	logger = log.NewLogger(dbusSender)
)

func Start() {
	if _manager != nil {
		return
	}

	logger.BeginTracing()

	var err error
	_manager, err = NewManager()
	if err != nil {
		logger.Error("Create Manager failed:", err)
		logger.EndTracing()
		return
	}

	err = dbus.InstallOnSession(_manager)
	if err != nil {
		logger.Error("Install DBus failed:", err)
		_manager.destroy()
		_manager = nil
		logger.EndTracing()
		return
	}
	_manager.handlePropChanged()
}

func Stop() {
	if _manager == nil {
		return
	}

	logger.EndTracing()
	_manager.destroy()
	_manager = nil
}