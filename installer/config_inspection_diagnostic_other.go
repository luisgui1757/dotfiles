//go:build !windows

package installer

func configInspectionIdentityDiagnostic(_ string) string { return "Windows-only probe" }
