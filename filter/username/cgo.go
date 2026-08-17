//go:build cgo

package filterusername

/*
#include <pwd.h>
#include <sys/types.h>
#include <stdlib.h>
*/
import "C"

import "fmt"

// FetchCgoPasswd returns passwd entries from the system via CGO getpwent.
func FetchCgoPasswd() ([]string, error) {
	var entries []string
	C.setpwent()
	defer C.endpwent()

	for {
		pw := C.getpwent()
		if pw == nil {
			break
		}
		// Format: name:passwd:uid:gid:gecos:dir:shell
		entry := fmt.Sprintf(
			"%s:x:%d:%d:%s:%s:%s",
			C.GoString(pw.pw_name),
			uint32(pw.pw_uid),
			uint32(pw.pw_gid),
			C.GoString(pw.pw_gecos),
			C.GoString(pw.pw_dir),
			C.GoString(pw.pw_shell),
		)
		entries = append(entries, entry)
	}
	return entries, nil
}

// cgoSource is the CGO-backed user database lookup, available only when this file
// is compiled (CGO enabled with a C toolchain present). It is nil in a non-CGO build.
var cgoSource func() ([]string, error) = FetchCgoPasswd
