// Copyright 2017 Yanni Coroneos. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

var Armhackmode uint32
var btrace = false

func gertproccount() int32 {
	return 4
}

func BTrace() int {
	btrace = true
	return 0
}
func brk() {
	func() {
		print("bkpt")
	}()
	for {
	}
}
