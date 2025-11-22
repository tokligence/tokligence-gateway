package patterns

import (
	"regexp"
	"testing"
)

// PIITestCase represents a test case for PII pattern matching
type PIITestCase struct {
	Name          string
	Pattern       *regexp.Regexp
	ShouldMatch   []string
	ShouldNotMatch []string
	Description   string
}

// TestGlobalPatterns tests global/universal PII patterns
func TestGlobalPatterns(t *testing.T) {
	tests := []PIITestCase{
		{
			Name:    "EMAIL",
			Pattern: regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
			ShouldMatch: []string{
				"john.doe@example.com",
				"user+tag@subdomain.example.org",
				"name_123@company.co.uk",
				"test.email@my-domain.com",
			},
			ShouldNotMatch: []string{
				"not-an-email",
				"@example.com",
				"user@",
				"user@.com",
			},
			Description: "Standard email address (RFC 5322)",
		},
		{
			Name:    "IP_ADDRESS_V4",
			Pattern: regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`),
			ShouldMatch: []string{
				"192.168.1.1",
				"10.0.0.1",
				"8.8.8.8",
				"255.255.255.255",
				"0.0.0.0",
			},
			ShouldNotMatch: []string{
				"256.1.1.1",
				"192.168.1",
				// Note: "1.2.3.4.5" will match "1.2.3.4" - known limitation without lookbehind
				"abc.def.ghi.jkl",
			},
			Description: "IPv4 address (note: may match version numbers like 1.2.3.4)",
		},
		{
			Name:    "URL",
			Pattern: regexp.MustCompile(`\b(?:https?://|www\.)[a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)+(?:/[^\s]*)?\b`),
			ShouldMatch: []string{
				"https://example.com",
				"http://test.org/path",
				"www.example.com",
				"https://sub.domain.example.com/path?query=value",
			},
			ShouldNotMatch: []string{
				"ftp://example.com",
				"example.com",
				"http://",
			},
			Description: "Web URL",
		},
		{
			Name:    "API_KEY_OPENAI",
			Pattern: regexp.MustCompile(`\bsk-[a-zA-Z0-9]{20,}\b`),
			ShouldMatch: []string{
				"sk-abc123def456ghi789jkl012mno345pqr",
				"sk-ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890",
			},
			ShouldNotMatch: []string{
				"sk-short",
				"sk-",
				"notsk-abc123def456ghi789jkl012",
			},
			Description: "OpenAI API key",
		},
		{
			Name:    "API_KEY_ANTHROPIC",
			Pattern: regexp.MustCompile(`\bsk-ant-[a-zA-Z0-9-]{20,}\b`),
			ShouldMatch: []string{
				"sk-ant-abc123-def456-ghi789-jkl012",
				"sk-ant-ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890",
			},
			ShouldNotMatch: []string{
				"sk-ant-short",
				"sk-ant-",
				"sk-abc123",
			},
			Description: "Anthropic API key",
		},
		{
			Name:    "API_KEY_AWS",
			Pattern: regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
			ShouldMatch: []string{
				"AKIAIOSFODNN7EXAMPLE",
				"ASIAXYZ123456789ABCD",  // Corrected: needs 16 chars after ASIA
			},
			ShouldNotMatch: []string{
				"AKIA123",
				"AKIAIOSFODNN7EXAMPLEextra",
				"akiaiosfodnn7example",
			},
			Description: "AWS access key (AKIA/ASIA + 16 alphanumeric)",
		},
		{
			Name:    "API_KEY_GITHUB",
			Pattern: regexp.MustCompile(`\bghp_[a-zA-Z0-9]{36,}\b`),
			ShouldMatch: []string{
				"ghp_abcdefghijklmnopqrstuvwxyz1234567890",
			},
			ShouldNotMatch: []string{
				"ghp_short",
				"ghp_",
			},
			Description: "GitHub personal access token",
		},
		{
			Name:    "CRYPTO_BTC",
			Pattern: regexp.MustCompile(`\b[13][a-km-zA-HJ-NP-Z1-9]{25,34}\b`),
			ShouldMatch: []string{
				"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
				"3J98t1WpEZ73CNmYviecrnyiWrnqRhWNLy",
			},
			ShouldNotMatch: []string{
				"2invalidbtcaddress",
				"1short",
			},
			Description: "Bitcoin address",
		},
		{
			Name:    "CRYPTO_ETH",
			Pattern: regexp.MustCompile(`\b0x[a-fA-F0-9]{40}\b`),
			ShouldMatch: []string{
				"0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0",
				"0xde0b295669a9fd93d5f28d9ec85e40f4cb697bae",
			},
			ShouldNotMatch: []string{
				"0x123",
				"0xGHIJKL",
			},
			Description: "Ethereum address",
		},
	}

	runTestCases(t, tests)
}

// TestUSPatterns tests US-specific PII patterns
func TestUSPatterns(t *testing.T) {
	tests := []PIITestCase{
		{
			Name:    "SSN",
			Pattern: regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			ShouldMatch: []string{
				"123-45-6789",
				"000-12-3456",
				"999-99-9999",
			},
			ShouldNotMatch: []string{
				"123456789",
				"123-456-789",
				"12-34-5678",
			},
			Description: "US Social Security Number",
		},
		{
			Name:    "PHONE_US",
			Pattern: regexp.MustCompile(`\b(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`),
			ShouldMatch: []string{
				"(555) 123-4567",
				"555-123-4567",
				"5551234567",
				"+1-555-123-4567",
				"1-555-123-4567",
				"555.123.4567",
			},
			ShouldNotMatch: []string{
				"12345",
				"555-12-345",
			},
			Description: "US phone number",
		},
		{
			Name:    "CREDIT_CARD",
			Pattern: regexp.MustCompile(`\b(?:(?:4\d{3}|5[1-5]\d{2}|6011)[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}|3[47]\d{2}[-\s]?\d{6}[-\s]?\d{5})\b`),
			ShouldMatch: []string{
				"4532-1234-5678-9010",
				"5425-2334-3010-9903",
				"6011-1111-1111-1117",
				"3782-822463-10005",
				"4532123456789010",
			},
			ShouldNotMatch: []string{
				"1234-5678-9012-3456",
				"4532-1234-5678",
			},
			Description: "Credit card number (Visa, MC, Amex, Discover)",
		},
		{
			Name:    "ITIN",
			Pattern: regexp.MustCompile(`\b9\d{2}-[78]\d-\d{4}\b`),
			ShouldMatch: []string{
				"912-70-1234",
				"900-80-9999",
			},
			ShouldNotMatch: []string{
				"123-70-1234",
				"912-90-1234",
			},
			Description: "US ITIN",
		},
	}

	runTestCases(t, tests)
}

// TestChinaPatterns tests China-specific PII patterns
func TestChinaPatterns(t *testing.T) {
	tests := []PIITestCase{
		{
			Name:    "NATIONAL_ID_CN",
			Pattern: regexp.MustCompile(`\b[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[0-9Xx]\b`),
			ShouldMatch: []string{
				"110101199001011234",
				"320106198506061234",
				"440305200001011234",
				"11010119900101123X",
			},
			ShouldNotMatch: []string{
				"12345678901234567",  // Too short
				"110101179001011234", // Invalid century
				"110101199013011234", // Invalid month
				"110101199001321234", // Invalid day
			},
			Description: "Chinese National ID (18 digits)",
		},
		{
			Name:    "PHONE_CN_MOBILE",
			Pattern: regexp.MustCompile(`\b1[3-9]\d{9}\b`),
			ShouldMatch: []string{
				"13812345678",
				"15912345678",
				"18612345678",
				"19912345678",
			},
			ShouldNotMatch: []string{
				"12812345678",
				"138123456",
				"138123456789",
			},
			Description: "Chinese mobile phone",
		},
		{
			Name:    "PHONE_CN_LANDLINE",
			Pattern: regexp.MustCompile(`\b0\d{2,3}-?\d{7,8}\b`),
			ShouldMatch: []string{
				"010-12345678",
				"021-1234567",
				"02112345678",
			},
			ShouldNotMatch: []string{
				"12345678",
				"010-123",
			},
			Description: "Chinese landline",
		},
		{
			Name:    "PASSPORT_CN",
			Pattern: regexp.MustCompile(`\b[EG]\d{8}\b`),
			ShouldMatch: []string{
				"E12345678",
				"G98765432",
			},
			ShouldNotMatch: []string{
				"A12345678",
				"E1234567",
			},
			Description: "Chinese passport",
		},
	}

	runTestCases(t, tests)
}

// TestEUPatterns tests EU-specific PII patterns
func TestEUPatterns(t *testing.T) {
	tests := []PIITestCase{
		{
			Name:    "IBAN",
			Pattern: regexp.MustCompile(`\b[A-Z]{2}\d{2}[A-Z0-9]{1,30}\b`),
			ShouldMatch: []string{
				"GB29NWBK60161331926819",
				"DE89370400440532013000",
				"FR1420041010050500013M02606",
			},
			ShouldNotMatch: []string{
				"1234NWBK60161331926819",
				"GB",
			},
			Description: "EU IBAN",
		},
	}

	runTestCases(t, tests)
}

// TestUKPatterns tests UK-specific PII patterns
func TestUKPatterns(t *testing.T) {
	tests := []PIITestCase{
		{
			Name:    "NATIONAL_INSURANCE_UK",
			Pattern: regexp.MustCompile(`\b[A-CEGHJ-PR-TW-Z]{1}[A-CEGHJ-NPR-TW-Z]{1}\d{6}[A-D]{1}\b`),
			ShouldMatch: []string{
				"AB123456C",
				"JR123456D",
			},
			ShouldNotMatch: []string{
				"AB123456E",
				"1B123456C",
			},
			Description: "UK National Insurance Number",
		},
	}

	runTestCases(t, tests)
}

// TestIndiaPatterns tests India-specific PII patterns
func TestIndiaPatterns(t *testing.T) {
	tests := []PIITestCase{
		{
			Name:    "PAN_IN",
			Pattern: regexp.MustCompile(`\b[A-Z]{5}\d{4}[A-Z]{1}\b`),
			ShouldMatch: []string{
				"ABCDE1234F",
				"ZYXWV9876G",
			},
			ShouldNotMatch: []string{
				"ABCD1234F",
				"ABCDE12345",
				"abcde1234f",
			},
			Description: "Indian PAN",
		},
	}

	runTestCases(t, tests)
}

// TestSingaporePatterns tests Singapore-specific PII patterns
func TestSingaporePatterns(t *testing.T) {
	tests := []PIITestCase{
		{
			Name:    "NRIC_SG",
			Pattern: regexp.MustCompile(`\b[STFG]\d{7}[A-Z]\b`),
			ShouldMatch: []string{
				"S1234567A",
				"T9876543Z",
				"F0123456B",
				"G8765432C",
			},
			ShouldNotMatch: []string{
				"A1234567B",
				"S123456A",
				"S12345678A",
			},
			Description: "Singapore NRIC/FIN",
		},
		{
			Name:    "PHONE_SG",
			Pattern: regexp.MustCompile(`\b(?:\+65\s?|65)?[689]\d{7}\b`),
			ShouldMatch: []string{
				"61234567",
				"81234567",
				"91234567",
				"+65 61234567",
				"+6561234567",
			},
			ShouldNotMatch: []string{
				"12345678",
				"512345678",
			},
			Description: "Singapore phone number",
		},
	}

	runTestCases(t, tests)
}

// Helper function to run test cases
func runTestCases(t *testing.T, tests []PIITestCase) {
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			// Test positive matches
			for _, text := range tc.ShouldMatch {
				if !tc.Pattern.MatchString(text) {
					t.Errorf("Pattern %s should match '%s' but didn't", tc.Name, text)
				}
			}

			// Test negative matches
			for _, text := range tc.ShouldNotMatch {
				if tc.Pattern.MatchString(text) {
					t.Errorf("Pattern %s should NOT match '%s' but did", tc.Name, text)
				}
			}
		})
	}
}

// BenchmarkPatterns benchmarks pattern matching performance
func BenchmarkPatterns(b *testing.B) {
	patterns := map[string]*regexp.Regexp{
		"EMAIL":      regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
		"PHONE_US":   regexp.MustCompile(`\b(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`),
		"SSN":        regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		"IP_V4":      regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`),
		"NATIONAL_ID_CN": regexp.MustCompile(`\b[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[0-9Xx]\b`),
	}

	testText := "My email is john@example.com, phone is 555-123-4567, SSN is 123-45-6789, IP is 192.168.1.100, and Chinese ID is 110101199001011234"

	for name, pattern := range patterns {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				pattern.FindString(testText)
			}
		})
	}
}
