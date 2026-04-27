package buildinfo

import (
	"os"
	"strings"
)

// IsContainer reports whether the process is running inside a container.
//
// The check is best-effort and informational only — it is not a security
// boundary. It returns true when any of the following conditions is satisfied:
//   - /.dockerenv exists (Docker).
//   - /run/.containerenv exists (Podman).
//   - The environment variable KUBERNETES_SERVICE_HOST is non-empty (Kubernetes).
//   - The environment variable RUNNING_IN_CONTAINER is non-empty (explicit opt-in
//     for image authors and orchestrators that cannot rely on filesystem markers).
//   - /proc/1/cgroup contains any of "docker", "containerd", or "kubepods".
//
// Filesystem errors degrade gracefully to false so the function never panics.
func IsContainer() bool {
	if fileExists("/.dockerenv") {
		return true
	}
	if fileExists("/run/.containerenv") {
		return true
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	if os.Getenv("RUNNING_IN_CONTAINER") != "" {
		return true
	}
	return cgroupMarker()
}

// fileExists returns true when path exists, regardless of its type.
// Permission errors and other filesystem failures return false.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// cgroupMarker reads /proc/1/cgroup and reports whether it contains a known
// container-runtime marker. Returns false on any I/O error.
func cgroupMarker() bool {
	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	content := string(data)
	for _, marker := range []string{"docker", "containerd", "kubepods"} {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}
