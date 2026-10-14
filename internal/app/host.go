package app

// Host describes the machine caged itself runs on, as system.Platform reports
// it. Arch is the architecture caged was built for.
type Host struct{ OS, Arch string }
