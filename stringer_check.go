package statuspage

import (
	"fmt"
	"reflect"
)

var stringerReflectType = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()

func eligibleStringer(t reflect.Type) bool {
	if !t.Implements(stringerReflectType) {
		// it's not a stringer
		return false
	}

	// We really don't need to import the protobuf library /just/ so we can
	// skip calling the String() method.
	// Instead, we'll manually check against the only publicly visible method on the proto.Message interface.
	if m, ok := t.MethodByName("ProtoReflect"); ok {
		if m.Type.NumOut() == 1 && m.Type.Out(0).PkgPath() == "google.golang.org/protobuf/reflect/protoreflect" {
			return false
		}
	}
	return true
}
