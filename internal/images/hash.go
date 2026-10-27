package images

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// ContextHash returns the hex SHA-256 of a build context: a version prefix,
// then every file sorted by name as name, mode (octal), length, and data,
// each followed by a NUL byte. It covers names, modes, and bytes, and does
// not depend on the order of files.
func ContextHash(files []plan.File) string {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(a, b plan.File) int { return cmp.Compare(a.Name, b.Name) })
	sum := sha256.New()
	sum.Write([]byte("caged-context/v1\x00"))
	for _, file := range sorted {
		sum.Write([]byte(file.Name + "\x00" + strconv.FormatUint(uint64(file.Mode), 8) + "\x00" + strconv.Itoa(len(file.Data)) + "\x00"))
		sum.Write(file.Data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}
