package config

import (
	"testing"
)

func TestIsPathAllowedEdgeCases(t *testing.T) {
	tests := []struct {
		name                 string
		path                 string
		allowedRoots         []string
		deniedPaths          []string
		overrideAllowedRoots []string
		expectedAllowed      bool
	}{
		// 1. Root path tests
		{name: "root_allowed_root_roots", path: "/", allowedRoots: []string{"/"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "root_denied_root_roots", path: "/", allowedRoots: []string{"/"}, deniedPaths: []string{"/"}, expectedAllowed: false},
		{name: "root_not_in_allowed_roots", path: "/", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},

		// 2. Allowed roots checks
		{name: "exact_allowed_path", path: "/var/log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "exact_allowed_path_with_trailing_slash", path: "/var/log/", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "allowed_path_roots_with_trailing_slash", path: "/var/log", allowedRoots: []string{"/var/log/"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "child_of_allowed_root", path: "/var/log/app.log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "deep_child_of_allowed_root", path: "/var/log/sub/dir/app.log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "parent_of_allowed_root_denied", path: "/var", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},
		{name: "prefix_matching_segment_boundary", path: "/var/log-alternate", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},
		{name: "prefix_matching_segment_boundary_child", path: "/var/log-alternate/app.log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},

		// 3. Deny-list priority tests
		{name: "exact_denied_path", path: "/var/log/secrets", allowedRoots: []string{"/var/log"}, deniedPaths: []string{"/var/log/secrets"}, expectedAllowed: false},
		{name: "child_of_denied_path", path: "/var/log/secrets/key.pem", allowedRoots: []string{"/var/log"}, deniedPaths: []string{"/var/log/secrets"}, expectedAllowed: false},
		{name: "sibling_of_denied_path_allowed", path: "/var/log/secrets-not", allowedRoots: []string{"/var/log"}, deniedPaths: []string{"/var/log/secrets"}, expectedAllowed: true},
		{name: "sibling_child_of_denied_path_allowed", path: "/var/log/secrets-not/key.pem", allowedRoots: []string{"/var/log"}, deniedPaths: []string{"/var/log/secrets"}, expectedAllowed: true},
		{name: "deny_root_blocks_all", path: "/var/log/app.log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{"/"}, expectedAllowed: false},
		{name: "deny_parent_blocks_child", path: "/var/log/app.log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{"/var"}, expectedAllowed: false},

		// 4. Path traversal tests
		{name: "traversal_attempt_dotdot", path: "/var/log/../../../etc/passwd", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},
		{name: "traversal_attempt_dotdot_prefix", path: "../etc/passwd", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},
		{name: "traversal_attempt_relative", path: "../../etc/passwd", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},
		{name: "traversal_resolving_to_allowed", path: "/var/log/../log/app.log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "traversal_with_dot_allowed", path: "/var/log/./app.log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "denied_traversal_resolving_to_denied", path: "/var/log/secrets/../secrets/key.pem", allowedRoots: []string{"/var/log"}, deniedPaths: []string{"/var/log/secrets"}, expectedAllowed: false},

		// 5. Empty and edge configuration scenarios
		{name: "empty_allowed_roots_denies_all", path: "/var/log", allowedRoots: []string{}, deniedPaths: []string{}, expectedAllowed: false},
		{name: "empty_allowed_roots_root_path", path: "/", allowedRoots: []string{}, deniedPaths: []string{}, expectedAllowed: false},
		{name: "empty_denylist_allowed", path: "/var/log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "empty_path", path: "", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},
		{name: "relative_path_no_slash", path: "var/log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},
		{name: "relative_dot_path", path: ".", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},

		// 6. Overrides of Allowed Roots
		{name: "override_active", path: "/var/log", allowedRoots: []string{"/etc"}, deniedPaths: []string{}, overrideAllowedRoots: []string{"/var/log"}, expectedAllowed: true},
		{name: "override_active_but_fails", path: "/etc/passwd", allowedRoots: []string{"/etc"}, deniedPaths: []string{}, overrideAllowedRoots: []string{"/var/log"}, expectedAllowed: false},
		{name: "override_empty_uses_configured", path: "/var/log", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, overrideAllowedRoots: []string{}, expectedAllowed: true},
		{name: "override_empty_uses_configured_fails", path: "/etc/passwd", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, overrideAllowedRoots: []string{}, expectedAllowed: false},

		// 7. Multiple entries in lists
		{name: "multi_allowed_match_first", path: "/etc/hosts", allowedRoots: []string{"/etc", "/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "multi_allowed_match_second", path: "/var/log/nginx", allowedRoots: []string{"/etc", "/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "multi_allowed_no_match", path: "/usr/local", allowedRoots: []string{"/etc", "/var/log"}, deniedPaths: []string{}, expectedAllowed: false},
		{name: "multi_denied_match_first", path: "/var/log/secrets/abc", allowedRoots: []string{"/var/log"}, deniedPaths: []string{"/var/log/secrets", "/var/log/private"}, expectedAllowed: false},
		{name: "multi_denied_match_second", path: "/var/log/private/def", allowedRoots: []string{"/var/log"}, deniedPaths: []string{"/var/log/secrets", "/var/log/private"}, expectedAllowed: false},
		{name: "multi_denied_no_match", path: "/var/log/public", allowedRoots: []string{"/var/log"}, deniedPaths: []string{"/var/log/secrets", "/var/log/private"}, expectedAllowed: true},

		// 8. Spaces and special characters
		{name: "path_with_spaces", path: "/var/log/my folder/file.txt", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "path_with_special_chars", path: "/var/log/$.txt", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "case_sensitivity_mismatch", path: "/var/LOG", allowedRoots: []string{"/var/log"}, deniedPaths: []string{}, expectedAllowed: false},

		// 9. Slash-only allowed roots
		{name: "slash_allowed_any_path", path: "/etc/passwd", allowedRoots: []string{"/"}, deniedPaths: []string{}, expectedAllowed: true},
		{name: "slash_allowed_denied_etc", path: "/etc/passwd", allowedRoots: []string{"/"}, deniedPaths: []string{"/etc"}, expectedAllowed: false},
		{name: "slash_allowed_denied_etc_allowed_var", path: "/var/log", allowedRoots: []string{"/"}, deniedPaths: []string{"/etc"}, expectedAllowed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AgentConfig{
				AllowedRoots: tt.allowedRoots,
				DeniedPaths:  tt.deniedPaths,
			}
			result := cfg.IsPathAllowed(tt.path, tt.overrideAllowedRoots)
			if result != tt.expectedAllowed {
				t.Errorf("IsPathAllowed(%q, %v) with Config{Allowed: %v, Denied: %v} = %v; want %v",
					tt.path, tt.overrideAllowedRoots, tt.allowedRoots, tt.deniedPaths, result, tt.expectedAllowed)
			}
		})
	}
}
