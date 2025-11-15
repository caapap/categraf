/* Copyright(C) 2025. Huawei Technologies Co.,Ltd. All rights reserved.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package ethtool provides the tools for ethernet
package ethtool

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestGetEthOperState(t *testing.T) {
	convey.Convey("TestGetLoOperState", t, func() {
		state, err := GetInterfaceOperState("lo")
		convey.So(state, convey.ShouldEqual, "unknown")
		convey.So(err, convey.ShouldEqual, nil)
	})

	convey.Convey("TestGetLoOperState", t, func() {
		state, err := GetInterfaceOperState("noexisteth")
		convey.So(state, convey.ShouldEqual, "")
		convey.So(err, convey.ShouldNotEqual, nil)
	})
}
