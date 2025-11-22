package fixtures

// PIITestData contains test data for PII pattern testing
type PIITestData struct {
	Text        string
	ExpectedPII map[string][]string // PII type -> list of expected values
}

// GlobalPIIExamples provides test data for global PII patterns
var GlobalPIIExamples = []PIITestData{
	{
		Text: "Contact me at john.doe@example.com or alice@company.org for more info.",
		ExpectedPII: map[string][]string{
			"EMAIL": {"john.doe@example.com", "alice@company.org"},
		},
	},
	{
		Text: "Server IPs: 192.168.1.100, 10.0.0.1, and 8.8.8.8",
		ExpectedPII: map[string][]string{
			"IP_ADDRESS": {"192.168.1.100", "10.0.0.1", "8.8.8.8"},
		},
	},
	{
		Text: "Visit https://example.com or www.test.org for details",
		ExpectedPII: map[string][]string{
			"URL": {"https://example.com", "www.test.org"},
		},
	},
	{
		Text: "My OpenAI key is sk-proj-abc123def456ghi789jkl012mno345pqr and Anthropic key is sk-ant-xyz789-uvw456-rst123",
		ExpectedPII: map[string][]string{
			"API_KEY": {"sk-proj-abc123def456ghi789jkl012mno345pqr", "sk-ant-xyz789-uvw456-rst123"},
		},
	},
	{
		Text: "Send Bitcoin to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa or Ethereum to 0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0",
		ExpectedPII: map[string][]string{
			"CRYPTO_ADDRESS": {"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0"},
		},
	},
}

// USPIIExamples provides test data for US PII patterns
var USPIIExamples = []PIITestData{
	{
		Text: "My SSN is 123-45-6789 and phone is (555) 123-4567",
		ExpectedPII: map[string][]string{
			"SSN":   {"123-45-6789"},
			"PHONE": {"(555) 123-4567"},
		},
	},
	{
		Text: "Credit card: 4532-1234-5678-9010, contact at 555-987-6543",
		ExpectedPII: map[string][]string{
			"CREDIT_CARD": {"4532-1234-5678-9010"},
			"PHONE":       {"555-987-6543"},
		},
	},
	{
		Text: "ITIN: 912-70-1234, call +1-555-123-4567",
		ExpectedPII: map[string][]string{
			"ITIN":  {"912-70-1234"},
			"PHONE": {"+1-555-123-4567"},
		},
	},
}

// ChinaPIIExamples provides test data for China PII patterns
var ChinaPIIExamples = []PIITestData{
	{
		Text: "我的身份证号是 110101199001011234，手机号是 13812345678",
		ExpectedPII: map[string][]string{
			"NATIONAL_ID": {"110101199001011234"},
			"PHONE":       {"13812345678"},
		},
	},
	{
		Text: "请拨打固定电话 010-12345678 或手机 15912345678 联系我",
		ExpectedPII: map[string][]string{
			"PHONE": {"010-12345678", "15912345678"},
		},
	},
	{
		Text: "护照号码：E12345678，身份证：320106198506061234",
		ExpectedPII: map[string][]string{
			"PASSPORT":    {"E12345678"},
			"NATIONAL_ID": {"320106198506061234"},
		},
	},
}

// MultiRegionExamples combines PII from multiple regions
var MultiRegionExamples = []PIITestData{
	{
		Text: "Email john@example.com, US SSN 123-45-6789, China ID 110101199001011234, phone 555-123-4567 or 13812345678",
		ExpectedPII: map[string][]string{
			"EMAIL":       {"john@example.com"},
			"SSN":         {"123-45-6789"},
			"NATIONAL_ID": {"110101199001011234"},
			"PHONE":       {"555-123-4567", "13812345678"},
		},
	},
	{
		Text: "Contact: alice@company.org, UK NI AB123456C, Singapore NRIC S1234567A, API key sk-proj-abc123def456ghi789",
		ExpectedPII: map[string][]string{
			"EMAIL":       {"alice@company.org"},
			"NATIONAL_ID": {"AB123456C", "S1234567A"},
			"API_KEY":     {"sk-proj-abc123def456ghi789"},
		},
	},
}

// EdgeCases contains tricky cases that might cause false positives/negatives
var EdgeCases = []PIITestData{
	{
		Text: "Version 1.2.3.4 is not an IP address",
		ExpectedPII: map[string][]string{
			// Version numbers should NOT be detected as IPs in ideal cases
			// but current simple regex will match it - this is a known limitation
		},
	},
	{
		Text: "The number 123456789 might look like SSN but isn't formatted correctly",
		ExpectedPII: map[string][]string{
			// Should NOT match SSN pattern (requires dashes)
		},
	},
	{
		Text: "Partial email user@ or @domain.com should not match",
		ExpectedPII: map[string][]string{
			// Should NOT match EMAIL pattern
		},
	},
}

// LongTextExample provides a realistic long text with multiple PII types
var LongTextExample = PIITestData{
	Text: `Dear Customer Service,

I'm writing to update my account information. My email address is john.doe@example.com
and I can also be reached at john.d@personal-email.org. My phone numbers are (555) 123-4567
(home) and 555-987-6543 (mobile).

For verification purposes:
- SSN: 123-45-6789
- Credit Card: 4532-1234-5678-9010
- Driver's License: CA D1234567

I recently moved and my new address includes my Chinese national ID 110101199001011234
for residency purposes. My Chinese mobile is 13812345678.

Please send confirmation to my Singapore NRIC S1234567A.

I'm also working on a tech project and need to update my API keys:
- OpenAI: sk-proj-abc123def456ghi789jkl012mno345pqr
- Anthropic: sk-ant-xyz789-uvw456-rst123
- AWS: AKIAIOSFODNN7EXAMPLE
- GitHub: ghp_abcdefghijklmnopqrstuvwxyz1234567890

My Bitcoin wallet is 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa and Ethereum address is
0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0.

Server details:
- Primary: 192.168.1.100
- Backup: 10.0.0.1
- DNS: 8.8.8.8

For more information, visit https://mycompany.com or www.support.example.org.

Best regards,
John Doe`,
	ExpectedPII: map[string][]string{
		"EMAIL":         {"john.doe@example.com", "john.d@personal-email.org"},
		"PHONE":         {"(555) 123-4567", "555-987-6543", "13812345678"},
		"SSN":           {"123-45-6789"},
		"CREDIT_CARD":   {"4532-1234-5678-9010"},
		"NATIONAL_ID":   {"110101199001011234", "S1234567A"},
		"API_KEY":       {"sk-proj-abc123def456ghi789jkl012mno345pqr", "sk-ant-xyz789-uvw456-rst123", "AKIAIOSFODNN7EXAMPLE", "ghp_abcdefghijklmnopqrstuvwxyz1234567890"},
		"CRYPTO_ADDRESS": {"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0"},
		"IP_ADDRESS":    {"192.168.1.100", "10.0.0.1", "8.8.8.8"},
		"URL":           {"https://mycompany.com", "www.support.example.org"},
	},
}

// RealWorldExamples contains anonymized real-world scenarios
var RealWorldExamples = []PIITestData{
	{
		Text: "Customer support ticket: User email: support@client.com Phone: +1-555-234-5678 Issue: Cannot access account",
		ExpectedPII: map[string][]string{
			"EMAIL": {"support@client.com"},
			"PHONE": {"+1-555-234-5678"},
		},
	},
	{
		Text: "Medical record #12345: Patient ID 110101199001011234, Contact: 13912345678, Email: patient@hospital.cn",
		ExpectedPII: map[string][]string{
			"NATIONAL_ID": {"110101199001011234"},
			"PHONE":       {"13912345678"},
			"EMAIL":       {"patient@hospital.cn"},
		},
	},
	{
		Text: "Legal document: SSN 987-65-4321, Credit Card ending in 9010 (4532-1234-5678-9010), Phone (555) 876-5432",
		ExpectedPII: map[string][]string{
			"SSN":         {"987-65-4321"},
			"CREDIT_CARD": {"4532-1234-5678-9010"},
			"PHONE":       {"(555) 876-5432"},
		},
	},
}
