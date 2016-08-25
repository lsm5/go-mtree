package mtree

// #include "vis.h"
// #include <stdlib.h>
import "C"
import (
	"fmt"
	"math"
	"unsafe"
)

// Vis is a wrapper of the C implementation of the function vis, which encodes
// a character with a particular format/style
func Vis(src string) (string, error) {
	// dst needs to be 4 times the length of str, must check appropriate size
	if uint32(len(src)*4+1) >= math.MaxUint32/4 {
		return "", fmt.Errorf("failed to encode: %q", src)
	}
	dst := string(make([]byte, 4*len(src)+1))
	cDst, cSrc := C.CString(dst), C.CString(src)
	defer C.free(unsafe.Pointer(cDst))
	defer C.free(unsafe.Pointer(cSrc))
	C.strvis(cDst, cSrc, C.int(VisWhite|VisOctal|VisGlob))

	return C.GoString(cDst), nil
}

type VisFlag int

const (
	// to select alternate encoding format
	VisOctal  VisFlag = 0x01 // use octal \ddd format
	VisCstyle VisFlag = 0x02 // use \[nrft0..] where appropriate

	// to alter set of characters encoded (default is to encode all non-graphic
	// except space, tab, and newline).
	VisSp    VisFlag = 0x04 // also encode space
	VisTab   VisFlag = 0x08 // also encode tab
	VisNl    VisFlag = 0x10 // also encode newline
	VisWhite VisFlag = (VIS_SP | VIS_TAB | VIS_NL)
	VisSafe  VisFlag = 0x20 // only encode "unsafe" characters

	// other
	VisNoslash   VisFlag = 0x40  // inhibit printing '\'
	VisHttpstyle VisFlag = 0x80  // http-style escape % HEX HEX
	VisGlob      VisFlag = 0x100 // encode glob(3) magics

	// unvis return codes
	UnvisErrorValid         UnvisError = 1  // character valid
	UnvisErrorValidpush     UnvisError = 2  // character valid, push back passed char
	UnvisErrorNochar        UnvisError = 3  // valid sequence, no character produced
	UnvisErrorSynbad        UnvisError = -1 // unrecognized escape sequence
	UnvisErrorUnrecoverable UnvisError = -2 // decoder in unknown state (unrecoverable)

	// unvis flags
	UnvisEnd VisFlag = 1 // no more characters
)

type UnvisError int

func (ue UnvisError) Error() string {
	switch ue {
	case UnvisErrorValid:
		return "character valid"
	case UnvisErrorValidPush:
		return "character valid, push back passed char"
	case UnvisErrorNochar:
		return "valid sequence, no character produced"
	case UnvisErrorSynbad:
		return "unrecognized escape sequence"
	case UnvisErrorUnrecoverable:
		return "decoder in unknown state (unrecoverable)"
	}
	return "Unknown Error"
}
