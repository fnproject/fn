package permbits

import (
	"os"
	"syscall"
)

type PermissionBits uint32

const (
	Setuid PermissionBits = 1 << (12 - 1 - iota)
	Setgid
	Sticky
	UserRead
	UserWrite
	UserExecute
	GroupRead
	GroupWrite
	GroupExecute
	OtherRead
	OtherWrite
	OtherExecute
)

// Given a filepath, get it's permission bits
func Stat(filepath string) (PermissionBits, error) {
	fi, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	return FileMode(fi.Mode()), nil
}

// Given a FileMode from the os package, get it's permission bits
func FileMode(fm os.FileMode) PermissionBits {
	perm := PermissionBits(fm.Perm())

	if fm&os.ModeSetuid != 0 {
		perm.SetSetuid(true)
	}
	if fm&os.ModeSetgid != 0 {
		perm.SetSetgid(true)
	}
	if fm&os.ModeSticky != 0 {
		perm.SetSticky(true)
	}
	return perm
}

// Given a filepath, set it's permission bits directly
func Chmod(filepath string, b PermissionBits) error {
	if e := syscall.Chmod(filepath, syscallMode(b)); e != nil {
		return &os.PathError{"chmod", filepath, e}
	}
	return nil
}

// Given an os.FileMode object, update it's permissions
func UpdateFileMode(fm *os.FileMode, b PermissionBits) {
	// Setuid, Setgid, and Sticky bits are not in the same position in the two bitmaks
	// So we need to set their values manually
	if b.Setuid() {
		*fm |= os.ModeSetuid
	} else {
		*fm &^= os.ModeSetuid
	}
	if b.Setgid() {
		*fm |= os.ModeSetgid
	} else {
		*fm &^= os.ModeSetgid
	}
	if b.Sticky() {
		*fm |= os.ModeSticky
	} else {
		*fm &^= os.ModeSticky
	}

	// unset bit-values that don't map to the same position in FileMode
	b.SetSetgid(false)
	b.SetSetuid(false)
	b.SetSticky(false)

	// Clear the permission bitss
	*fm &^= 0777

	// Set the permission bits
	*fm |= os.FileMode(b)
}

func (b PermissionBits) Setuid() bool {
	return b&Setuid != 0
}

func (b PermissionBits) Setgid() bool {
	return b&Setgid != 0
}

func (b PermissionBits) Sticky() bool {
	return b&Sticky != 0
}

func (b PermissionBits) UserRead() bool {
	return b&UserRead != 0
}

func (b PermissionBits) UserWrite() bool {
	return b&UserWrite != 0
}

func (b PermissionBits) UserExecute() bool {
	return b&UserExecute != 0
}

func (b PermissionBits) GroupRead() bool {
	return b&GroupRead != 0
}

func (b PermissionBits) GroupWrite() bool {
	return b&GroupWrite != 0
}

func (b PermissionBits) GroupExecute() bool {
	return b&GroupExecute != 0
}

func (b PermissionBits) OtherRead() bool {
	return b&GroupRead != 0
}

func (b PermissionBits) OtherWrite() bool {
	return b&GroupWrite != 0
}

func (b PermissionBits) OtherExecute() bool {
	return b&GroupExecute != 0
}

func (b *PermissionBits) SetSetuid(set bool) {
	if set {
		*b |= Setuid
	} else {
		*b &^= Setuid
	}
}

func (b *PermissionBits) SetSetgid(set bool) {
	if set {
		*b |= Setgid
	} else {
		*b &^= Setgid
	}
}

func (b *PermissionBits) SetSticky(set bool) {
	if set {
		*b |= Sticky
	} else {
		*b &^= Sticky
	}
}

func (b *PermissionBits) SetUserRead(set bool) {
	if set {
		*b |= UserRead
	} else {
		*b &^= UserRead
	}
}

func (b *PermissionBits) SetUserWrite(set bool) {
	if set {
		*b |= UserWrite
	} else {
		*b &^= UserWrite
	}
}

func (b *PermissionBits) SetUserExecute(set bool) {
	if set {
		*b |= UserExecute
	} else {
		*b &^= UserExecute
	}
}

func (b *PermissionBits) SetGroupRead(set bool) {
	if set {
		*b |= GroupRead
	} else {
		*b &^= GroupRead
	}
}

func (b *PermissionBits) SetGroupWrite(set bool) {
	if set {
		*b |= GroupWrite
	} else {
		*b &^= GroupWrite
	}
}

func (b *PermissionBits) SetGroupExecute(set bool) {
	if set {
		*b |= GroupExecute
	} else {
		*b &^= GroupExecute
	}
}

func (b *PermissionBits) SetOtherRead(set bool) {
	if set {
		*b |= OtherRead
	} else {
		*b &^= OtherRead
	}
}

func (b *PermissionBits) SetOtherWrite(set bool) {
	if set {
		*b |= OtherWrite
	} else {
		*b &^= OtherWrite
	}
}

func (b *PermissionBits) SetOtherExecute(set bool) {
	if set {
		*b |= OtherExecute
	} else {
		*b &^= OtherExecute
	}
}

func (b PermissionBits) String() string {
	var buf [32]byte // Mode is uint32.
	w := 0

	const rwx = "rwxrwxrwx"
	for i, c := range rwx {
		if b&(1<<uint(9-1-i)) != 0 {
			buf[w] = byte(c)
		} else {
			buf[w] = '-'
		}
		w++
	}
	return string(buf[:w])
}

// syscallMode returns the syscall-specific mode bits from PermissionBits bit positions
func syscallMode(p PermissionBits) (o uint32) {
	o |= uint32(p)

	if p.Setuid() {
		o |= syscall.S_ISUID
	}
	if p.Setgid() {
		o |= syscall.S_ISGID
	}
	if p.Sticky() {
		o |= syscall.S_ISVTX
	}
	return
}
