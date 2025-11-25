"""
API Key Recognizer for Presidio

Detects API keys, tokens, and secrets from 30+ providers including:
- Cloud: AWS, Azure, Google Cloud, Alibaba Cloud, IBM Cloud, Oracle Cloud
- AI/ML: OpenAI, Anthropic, Hugging Face, Cohere, Replicate
- DevOps: GitHub, GitLab, Bitbucket, CircleCI, Jenkins
- Communications: Slack, Discord, Twilio, SendGrid, Mailchimp
- Payments: Stripe, Square, PayPal, Shopify
- And many more...

Based on patterns from:
- GitHub Secret Scanning: https://docs.github.com/en/code-security/secret-scanning
- Gitleaks: https://github.com/gitleaks/gitleaks
- secrets-patterns-db: https://github.com/mazen160/secrets-patterns-db
- TruffleHog: https://github.com/trufflesecurity/trufflehog

Detection methods:
1. Regex pattern matching for structured secrets (e.g., sk-proj-*, AKIA*)
2. Entropy analysis for high-randomness strings
3. Keyword context matching (e.g., "api_key", "secret", "token")
"""

import re
import math
import logging
from typing import List, Optional, Dict, Any
from presidio_analyzer import EntityRecognizer, RecognizerResult

logger = logging.getLogger(__name__)


# API Key patterns organized by provider
# Format: (pattern_name, regex, confidence, description)
API_KEY_PATTERNS = [
    # ============================================================================
    # AI/ML Providers
    # ============================================================================

    # OpenAI - Multiple formats
    # Old format: sk-[48 chars] containing "T3BlbkFJ" (base64 "OpenAI")
    # New format: sk-proj-*, sk-svcacct-*, sk-admin-*
    ("OPENAI_API_KEY",
     r"\b(sk-(?:proj-|svcacct-|admin-)?[A-Za-z0-9_-]{20,}T3BlbkFJ[A-Za-z0-9_-]{20,})\b",
     0.95, "OpenAI API Key (with T3BlbkFJ marker)"),

    # OpenAI new project key format (2024+)
    ("OPENAI_PROJECT_KEY",
     r"\b(sk-proj-[A-Za-z0-9_-]{48,156})\b",
     0.90, "OpenAI Project API Key"),

    # OpenAI service account key (various lengths)
    ("OPENAI_SVCACCT_KEY",
     r"\b(sk-svcacct-[A-Za-z0-9_-]{20,156})\b",
     0.90, "OpenAI Service Account Key"),

    # OpenAI admin key (various lengths)
    ("OPENAI_ADMIN_KEY",
     r"\b(sk-admin-[A-Za-z0-9_-]{20,156})\b",
     0.90, "OpenAI Admin Key"),

    # Anthropic Claude API Key
    ("ANTHROPIC_API_KEY",
     r"\b(sk-ant-api03-[a-zA-Z0-9_\-]{93}AA)\b",
     0.95, "Anthropic Claude API Key"),

    # Anthropic (shorter format)
    ("ANTHROPIC_API_KEY_SHORT",
     r"\b(sk-ant-[a-zA-Z0-9_\-]{32,100})\b",
     0.85, "Anthropic API Key"),

    # Hugging Face
    ("HUGGINGFACE_TOKEN",
     r"\b(hf_[a-zA-Z0-9]{34,})\b",
     0.95, "Hugging Face Access Token"),

    # Cohere
    ("COHERE_API_KEY",
     r"\b([a-zA-Z0-9]{40})\b",  # Will be combined with keyword matching
     0.70, "Cohere API Key (needs context)"),

    # Replicate
    ("REPLICATE_API_TOKEN",
     r"\b(r8_[a-zA-Z0-9]{37})\b",
     0.95, "Replicate API Token"),

    # Google AI (Gemini, PaLM)
    ("GOOGLE_AI_KEY",
     r"\b(AIza[A-Za-z0-9_-]{35})\b",
     0.95, "Google AI/Cloud API Key"),

    # ============================================================================
    # Cloud Providers
    # ============================================================================

    # AWS Access Key ID
    ("AWS_ACCESS_KEY_ID",
     r"\b((?:A3T[A-Z0-9]|AKIA|ASIA|ABIA|ACCA)[A-Z0-9]{16})\b",
     0.95, "AWS Access Key ID"),

    # AWS Secret Access Key (40 chars, base64-like)
    ("AWS_SECRET_ACCESS_KEY",
     r"(?i)(?:aws.{0,20}secret|secret.{0,20}key)['\"]?\s*[:=]\s*['\"]?([A-Za-z0-9/+=]{40})['\"]?",
     0.90, "AWS Secret Access Key"),

    # Azure
    ("AZURE_CLIENT_SECRET",
     r"(?:^|['\"\x60\s>=:(,)])([a-zA-Z0-9_~.]{3}\dQ~[a-zA-Z0-9_~.-]{31,34})",
     0.90, "Azure AD Client Secret"),

    ("AZURE_STORAGE_KEY",
     r"\b([A-Za-z0-9+/]{86}==)\b",
     0.80, "Azure Storage Account Key"),

    # Google Cloud Service Account
    ("GCP_SERVICE_ACCOUNT",
     r'"type"\s*:\s*"service_account"',
     0.95, "Google Cloud Service Account JSON"),

    # Google OAuth Client Secret
    ("GOOGLE_OAUTH_SECRET",
     r"\b(GOCSPX-[a-zA-Z0-9_-]{28})\b",
     0.95, "Google OAuth Client Secret"),

    # Alibaba Cloud
    ("ALIBABA_ACCESS_KEY",
     r"\b(LTAI[A-Za-z0-9]{20})\b",
     0.95, "Alibaba Cloud Access Key"),

    # IBM Cloud
    ("IBM_CLOUD_KEY",
     r"\b([a-zA-Z0-9_-]{44})\b",  # Combined with keyword
     0.70, "IBM Cloud API Key (needs context)"),

    # DigitalOcean
    ("DIGITALOCEAN_TOKEN",
     r"\b(dop_v1_[a-f0-9]{64})\b",
     0.95, "DigitalOcean Personal Access Token"),

    ("DIGITALOCEAN_OAUTH",
     r"\b(doo_v1_[a-f0-9]{64})\b",
     0.95, "DigitalOcean OAuth Token"),

    # Cloudflare
    ("CLOUDFLARE_API_KEY",
     r"\b([a-z0-9]{37})\b",  # Combined with keyword
     0.70, "Cloudflare API Key (needs context)"),

    ("CLOUDFLARE_API_TOKEN",
     r"\b([A-Za-z0-9_-]{40})\b",  # Combined with keyword
     0.70, "Cloudflare API Token (needs context)"),

    # Heroku
    ("HEROKU_API_KEY",
     r"\b([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})\b",
     0.75, "Heroku API Key (UUID format)"),

    # ============================================================================
    # DevOps & Version Control
    # ============================================================================

    # GitHub
    ("GITHUB_PAT",
     r"\b(ghp_[0-9a-zA-Z]{36})\b",
     0.95, "GitHub Personal Access Token"),

    ("GITHUB_OAUTH",
     r"\b(gho_[0-9a-zA-Z]{36})\b",
     0.95, "GitHub OAuth Token"),

    ("GITHUB_APP",
     r"\b((?:ghu|ghs)_[0-9a-zA-Z]{36})\b",
     0.95, "GitHub App Token"),

    ("GITHUB_REFRESH",
     r"\b(ghr_[0-9a-zA-Z]{36})\b",
     0.95, "GitHub Refresh Token"),

    ("GITHUB_FINE_GRAINED",
     r"\b(github_pat_[0-9a-zA-Z_]{22,82})\b",
     0.95, "GitHub Fine-grained PAT"),

    # GitLab
    ("GITLAB_PAT",
     r"\b(glpat-[0-9a-zA-Z_-]{20})\b",
     0.95, "GitLab Personal Access Token"),

    ("GITLAB_PIPELINE",
     r"\b(glptt-[0-9a-f]{40})\b",
     0.95, "GitLab Pipeline Trigger Token"),

    ("GITLAB_RUNNER",
     r"\b(GR1348941[0-9a-zA-Z_-]{20})\b",
     0.95, "GitLab Runner Registration Token"),

    # Bitbucket
    ("BITBUCKET_APP_PASSWORD",
     r"\b(ATBB[a-zA-Z0-9]{32})\b",
     0.95, "Bitbucket App Password"),

    # npm
    ("NPM_TOKEN",
     r"\b(npm_[a-zA-Z0-9]{36})\b",
     0.95, "npm Access Token"),

    # PyPI
    ("PYPI_TOKEN",
     r"\b(pypi-[A-Za-z0-9_-]{100,})\b",
     0.95, "PyPI API Token"),

    # Docker Hub
    ("DOCKER_TOKEN",
     r"\b(dckr_pat_[a-zA-Z0-9_-]{27})\b",
     0.95, "Docker Hub Personal Access Token"),

    # CircleCI
    ("CIRCLECI_TOKEN",
     r"\b([a-f0-9]{40})\b",  # Combined with keyword
     0.70, "CircleCI Token (needs context)"),

    # Travis CI
    ("TRAVIS_TOKEN",
     r"\b([a-zA-Z0-9_-]{22})\b",  # Combined with keyword
     0.70, "Travis CI Token (needs context)"),

    # ============================================================================
    # Communications & Messaging
    # ============================================================================

    # Slack
    ("SLACK_BOT_TOKEN",
     r"\b(xoxb-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*)\b",
     0.95, "Slack Bot Token"),

    ("SLACK_USER_TOKEN",
     r"\b(xoxp-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*)\b",
     0.95, "Slack User Token"),

    ("SLACK_APP_TOKEN",
     r"\b(xapp-[0-9]-[A-Z0-9]+-[0-9]+-[a-z0-9]+)\b",
     0.95, "Slack App Token"),

    ("SLACK_WEBHOOK",
     r"(https?://hooks\.slack\.com/(?:services|workflows|triggers)/[A-Za-z0-9+/]{43,56})",
     0.95, "Slack Webhook URL"),

    # Discord
    ("DISCORD_BOT_TOKEN",
     r"\b([MN][A-Za-z\d]{23,}\.[\w-]{6}\.[\w-]{27})\b",
     0.90, "Discord Bot Token"),

    ("DISCORD_WEBHOOK",
     r"(https?://(?:ptb\.|canary\.)?discord(?:app)?\.com/api/webhooks/[0-9]+/[A-Za-z0-9_-]+)",
     0.95, "Discord Webhook URL"),

    # Twilio
    ("TWILIO_API_KEY",
     r"\b(SK[0-9a-fA-F]{32})\b",
     0.95, "Twilio API Key"),

    ("TWILIO_ACCOUNT_SID",
     r"\b(AC[a-zA-Z0-9]{32})\b",
     0.95, "Twilio Account SID"),

    # Telegram
    ("TELEGRAM_BOT_TOKEN",
     r"\b([0-9]{8,10}:[a-zA-Z0-9_-]{35})\b",
     0.90, "Telegram Bot Token"),

    # ============================================================================
    # Email Services
    # ============================================================================

    # SendGrid
    ("SENDGRID_API_KEY",
     r"\b(SG\.[a-zA-Z0-9_-]{22}\.[a-zA-Z0-9_-]{43})\b",
     0.95, "SendGrid API Key"),

    # Mailchimp
    ("MAILCHIMP_API_KEY",
     r"\b([a-f0-9]{32}-us[0-9]{1,2})\b",
     0.95, "Mailchimp API Key"),

    # Mailgun
    ("MAILGUN_API_KEY",
     r"\b(key-[a-zA-Z0-9]{32})\b",
     0.95, "Mailgun API Key"),

    # Postmark
    ("POSTMARK_SERVER_TOKEN",
     r"\b([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})\b",
     0.75, "Postmark Server Token (UUID format)"),

    # ============================================================================
    # Payment Providers
    # ============================================================================

    # Stripe
    ("STRIPE_SECRET_KEY",
     r"\b(sk_(?:test|live|prod)_[a-zA-Z0-9]{10,99})\b",
     0.95, "Stripe Secret Key"),

    ("STRIPE_PUBLISHABLE_KEY",
     r"\b(pk_(?:test|live|prod)_[a-zA-Z0-9]{10,99})\b",
     0.85, "Stripe Publishable Key"),

    ("STRIPE_RESTRICTED_KEY",
     r"\b(rk_(?:test|live|prod)_[a-zA-Z0-9]{10,99})\b",
     0.95, "Stripe Restricted Key"),

    ("STRIPE_WEBHOOK_SECRET",
     r"\b(whsec_[a-zA-Z0-9]{32,})\b",
     0.95, "Stripe Webhook Secret"),

    # Square
    ("SQUARE_ACCESS_TOKEN",
     r"\b(sq0atp-[a-zA-Z0-9_-]{22})\b",
     0.95, "Square Access Token"),

    ("SQUARE_OAUTH_SECRET",
     r"\b(sq0csp-[a-zA-Z0-9_-]{43})\b",
     0.95, "Square OAuth Secret"),

    # PayPal
    ("PAYPAL_BRAINTREE",
     r"\b(access_token\$production\$[0-9a-z]{16}\$[0-9a-f]{32})\b",
     0.95, "PayPal Braintree Access Token"),

    # Shopify
    ("SHOPIFY_ACCESS_TOKEN",
     r"\b(shpat_[a-fA-F0-9]{32})\b",
     0.95, "Shopify Admin API Token"),

    ("SHOPIFY_CUSTOM_APP",
     r"\b(shpca_[a-fA-F0-9]{32})\b",
     0.95, "Shopify Custom App Token"),

    ("SHOPIFY_PRIVATE_APP",
     r"\b(shppa_[a-fA-F0-9]{32})\b",
     0.95, "Shopify Private App Token"),

    ("SHOPIFY_SHARED_SECRET",
     r"\b(shpss_[a-fA-F0-9]{32})\b",
     0.95, "Shopify Shared Secret"),

    # ============================================================================
    # Databases & Data Services
    # ============================================================================

    # MongoDB
    ("MONGODB_CONNECTION",
     r"mongodb(?:\+srv)?://[^\s\"']+:[^\s\"']+@[^\s\"']+",
     0.90, "MongoDB Connection String"),

    # PostgreSQL
    ("POSTGRES_CONNECTION",
     r"postgres(?:ql)?://[^\s\"']+:[^\s\"']+@[^\s\"']+",
     0.90, "PostgreSQL Connection String"),

    # MySQL
    ("MYSQL_CONNECTION",
     r"mysql://[^\s\"']+:[^\s\"']+@[^\s\"']+",
     0.90, "MySQL Connection String"),

    # Redis
    ("REDIS_CONNECTION",
     r"redis://[^\s\"']*:[^\s\"']+@[^\s\"']+",
     0.90, "Redis Connection String"),

    # Firebase
    ("FIREBASE_KEY",
     r"\b(AAAA[A-Za-z0-9_-]{7}:[A-Za-z0-9_-]{140})\b",
     0.95, "Firebase Cloud Messaging Key"),

    # Supabase
    ("SUPABASE_KEY",
     r"\b(sbp_[a-f0-9]{40})\b",
     0.95, "Supabase Service Key"),

    # PlanetScale
    ("PLANETSCALE_PASSWORD",
     r"\b(pscale_pw_[a-zA-Z0-9_-]{43})\b",
     0.95, "PlanetScale Password"),

    # ============================================================================
    # Monitoring & Analytics
    # ============================================================================

    # Datadog
    ("DATADOG_API_KEY",
     r"\b([a-f0-9]{32})\b",  # Combined with keyword
     0.70, "Datadog API Key (needs context)"),

    # New Relic
    ("NEWRELIC_LICENSE_KEY",
     r"\b([a-f0-9]{40})\b",  # Combined with keyword
     0.70, "New Relic License Key (needs context)"),

    # Sentry
    ("SENTRY_DSN",
     r"(https://[a-f0-9]+@(?:[a-z0-9]+\.)?sentry\.io/[0-9]+)",
     0.95, "Sentry DSN"),

    ("SENTRY_AUTH_TOKEN",
     r"\b(sntrys_[a-zA-Z0-9]{64})\b",
     0.95, "Sentry Auth Token"),

    # Segment
    ("SEGMENT_WRITE_KEY",
     r"\b([a-zA-Z0-9]{32})\b",  # Combined with keyword
     0.70, "Segment Write Key (needs context)"),

    # Mixpanel
    ("MIXPANEL_TOKEN",
     r"\b([a-f0-9]{32})\b",  # Combined with keyword
     0.70, "Mixpanel Token (needs context)"),

    # ============================================================================
    # Social & Auth Providers
    # ============================================================================

    # Facebook
    ("FACEBOOK_ACCESS_TOKEN",
     r"\b(EAA[a-zA-Z0-9]{100,})\b",
     0.90, "Facebook Access Token"),

    # Twitter/X
    ("TWITTER_BEARER_TOKEN",
     r"\b(AAAA[A-Za-z0-9%]{40,})\b",
     0.85, "Twitter Bearer Token"),

    # LinkedIn
    ("LINKEDIN_CLIENT_SECRET",
     r"\b([a-zA-Z0-9]{16})\b",  # Combined with keyword
     0.70, "LinkedIn Client Secret (needs context)"),

    # Auth0
    ("AUTH0_CLIENT_SECRET",
     r"\b([a-zA-Z0-9_-]{64})\b",  # Combined with keyword
     0.70, "Auth0 Client Secret (needs context)"),

    # Okta
    ("OKTA_API_TOKEN",
     r"\b(00[a-zA-Z0-9_-]{40})\b",
     0.90, "Okta API Token"),

    # ============================================================================
    # Infrastructure & DevOps
    # ============================================================================

    # Kubernetes
    ("KUBERNETES_SECRET",
     r"\b(eyJhbGciOiJSUzI1NiIsI[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+)\b",
     0.85, "Kubernetes Service Account Token"),

    # HashiCorp Vault
    ("VAULT_TOKEN",
     r"\b(hvs\.[a-zA-Z0-9_-]{24,})\b",
     0.95, "HashiCorp Vault Token"),

    ("VAULT_BATCH_TOKEN",
     r"\b(hvb\.[a-zA-Z0-9_-]{24,})\b",
     0.95, "HashiCorp Vault Batch Token"),

    # Terraform Cloud
    ("TERRAFORM_CLOUD_TOKEN",
     r"\b([a-zA-Z0-9]{14}\.atlasv1\.[a-zA-Z0-9]{67})\b",
     0.95, "Terraform Cloud Token"),

    # Pulumi
    ("PULUMI_ACCESS_TOKEN",
     r"\b(pul-[a-f0-9]{40})\b",
     0.95, "Pulumi Access Token"),

    # Netlify
    ("NETLIFY_ACCESS_TOKEN",
     r"(?i)(?:netlify)['\"]?\s*[:=]\s*['\"]?([a-z0-9=_\-]{40,46})",
     0.85, "Netlify Access Token"),

    # Vercel
    ("VERCEL_ACCESS_TOKEN",
     r"\b([a-zA-Z0-9]{24})\b",  # Combined with keyword
     0.70, "Vercel Access Token (needs context)"),

    # ============================================================================
    # Generic Private Keys
    # ============================================================================

    # RSA Private Key
    ("RSA_PRIVATE_KEY",
     r"-----BEGIN (?:RSA )?PRIVATE KEY-----",
     0.95, "RSA Private Key"),

    # SSH Private Key
    ("SSH_PRIVATE_KEY",
     r"-----BEGIN OPENSSH PRIVATE KEY-----",
     0.95, "OpenSSH Private Key"),

    # PGP Private Key
    ("PGP_PRIVATE_KEY",
     r"-----BEGIN PGP PRIVATE KEY BLOCK-----",
     0.95, "PGP Private Key"),

    # Generic Private Key
    ("GENERIC_PRIVATE_KEY",
     r"-----BEGIN (?:EC |DSA |ENCRYPTED )?PRIVATE KEY-----",
     0.95, "Generic Private Key"),

    # ============================================================================
    # Generic API Key Patterns (lower confidence, need context)
    # ============================================================================

    # Generic Bearer Token
    ("BEARER_TOKEN",
     r"(?i)bearer\s+([a-zA-Z0-9_\-\.=]{20,})",
     0.80, "Bearer Token"),

    # Generic Basic Auth
    ("BASIC_AUTH",
     r"(?i)basic\s+([a-zA-Z0-9+/=]{20,})",
     0.80, "Basic Auth Credentials"),

    # JWT Token
    ("JWT_TOKEN",
     r"\b(eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*)\b",
     0.85, "JWT Token"),
]

# Keywords that indicate API key context (for generic patterns)
API_KEY_KEYWORDS = [
    # Direct identifiers
    "api_key", "apikey", "api-key", "api_token", "apitoken", "api-token",
    "access_key", "accesskey", "access-key", "access_token", "accesstoken", "access-token",
    "secret_key", "secretkey", "secret-key", "secret_token", "secrettoken", "secret-token",
    "auth_key", "authkey", "auth-key", "auth_token", "authtoken", "auth-token",
    "private_key", "privatekey", "private-key",
    "client_secret", "clientsecret", "client-secret",
    "app_secret", "appsecret", "app-secret",
    "password", "passwd", "pwd",
    "credential", "credentials", "creds",
    "token", "bearer", "oauth",

    # Provider-specific keywords
    "openai", "anthropic", "claude", "gpt",
    "aws", "amazon", "azure", "google", "gcp",
    "github", "gitlab", "bitbucket",
    "stripe", "paypal", "square", "shopify",
    "slack", "discord", "twilio", "telegram",
    "sendgrid", "mailchimp", "mailgun",
    "datadog", "newrelic", "sentry",
    "firebase", "supabase", "mongodb", "postgres", "redis",
    "heroku", "netlify", "vercel", "cloudflare",
    "huggingface", "hugging_face", "hf_token",
]


def calculate_entropy(text: str) -> float:
    """
    Calculate Shannon entropy of a string.
    Higher entropy indicates more randomness (likely a secret).

    Typical values:
    - English text: 3.5-4.5 bits
    - Random alphanumeric: 5.5-6.0 bits
    - API keys/secrets: 5.0-6.0 bits
    """
    if not text:
        return 0.0

    # Count character frequencies
    freq = {}
    for char in text:
        freq[char] = freq.get(char, 0) + 1

    # Calculate entropy
    length = len(text)
    entropy = 0.0
    for count in freq.values():
        prob = count / length
        entropy -= prob * math.log2(prob)

    return entropy


class APIKeyRecognizer(EntityRecognizer):
    """
    Recognizer for detecting API keys, tokens, and secrets from 30+ providers.

    Uses multiple detection strategies:
    1. Specific regex patterns for known providers (high confidence)
    2. Generic patterns with keyword context (medium confidence)
    3. Entropy analysis for high-randomness strings (lower confidence)

    Example usage:
        recognizer = APIKeyRecognizer()
        results = recognizer.analyze("My key is sk-proj-abc123...", entities=["API_KEY"])
    """

    # Entity type for all API keys
    ENTITY_TYPE = "API_KEY"

    # Minimum entropy threshold for generic detection
    MIN_ENTROPY = 4.5

    # Minimum length for generic high-entropy strings
    MIN_LENGTH = 20

    def __init__(
        self,
        supported_language: str = "en",
        supported_entities: List[str] = None,
        min_confidence: float = 0.5,
        enable_entropy_detection: bool = True,
        enable_keyword_boost: bool = True,
    ):
        """
        Initialize the API Key Recognizer.

        Args:
            supported_language: Language code (default "en", works for all)
            supported_entities: List of entity types to detect
            min_confidence: Minimum confidence threshold (0.0-1.0)
            enable_entropy_detection: Enable entropy-based generic detection
            enable_keyword_boost: Boost confidence when keywords are present
        """
        supported_entities = supported_entities or [self.ENTITY_TYPE]

        super().__init__(
            supported_entities=supported_entities,
            supported_language=supported_language,
            name="APIKeyRecognizer",
        )

        self.min_confidence = min_confidence
        self.enable_entropy_detection = enable_entropy_detection
        self.enable_keyword_boost = enable_keyword_boost

        # Compile regex patterns
        self.compiled_patterns = []
        for name, pattern, confidence, description in API_KEY_PATTERNS:
            try:
                compiled = re.compile(pattern, re.IGNORECASE if not any(
                    c.isupper() for c in pattern.replace(r'\b', '').replace(r'\s', '')[:10]
                ) else 0)
                self.compiled_patterns.append((name, compiled, confidence, description))
            except re.error as e:
                # Log pattern compile error without potentially sensitive pattern names
                logger.warning(f"Failed to compile API key detection pattern: {type(e).__name__}")

        # Compile keyword pattern for context detection
        keyword_pattern = '|'.join(re.escape(kw) for kw in API_KEY_KEYWORDS)
        self.keyword_regex = re.compile(keyword_pattern, re.IGNORECASE)

        logger.info(f"APIKeyRecognizer initialized with {len(self.compiled_patterns)} patterns")

    def load(self):
        """Load method required by Presidio."""
        pass

    def get_supported_entities(self) -> List[str]:
        """Return list of supported entity types."""
        return self.supported_entities

    def _has_keyword_context(self, text: str, start: int, end: int, window: int = 50) -> bool:
        """
        Check if there's a keyword near the match that indicates API key context.

        Args:
            text: Full text being analyzed
            start: Start position of match
            end: End position of match
            window: Characters to check before and after

        Returns:
            True if keyword context is found
        """
        context_start = max(0, start - window)
        context_end = min(len(text), end + window)
        context = text[context_start:context_end].lower()

        return bool(self.keyword_regex.search(context))

    def _detect_high_entropy_strings(self, text: str) -> List[RecognizerResult]:
        """
        Detect potential secrets based on high entropy.

        Args:
            text: Text to analyze

        Returns:
            List of RecognizerResult for high-entropy strings
        """
        results = []

        # Pattern for potential secrets (alphanumeric + common special chars)
        potential_secret = re.compile(r'\b[A-Za-z0-9_\-+/=]{20,100}\b')

        for match in potential_secret.finditer(text):
            candidate = match.group()
            entropy = calculate_entropy(candidate)

            if entropy >= self.MIN_ENTROPY:
                # Check for keyword context to boost confidence
                has_context = self._has_keyword_context(text, match.start(), match.end())

                if has_context:
                    confidence = min(0.75, 0.5 + (entropy - self.MIN_ENTROPY) * 0.1)
                    results.append(RecognizerResult(
                        entity_type=self.ENTITY_TYPE,
                        start=match.start(),
                        end=match.end(),
                        score=confidence,
                        analysis_explanation=self._create_explanation(
                            f"High entropy string (entropy={entropy:.2f}) with keyword context"
                        ),
                    ))

        return results

    def _create_explanation(self, reason: str) -> Dict[str, Any]:
        """Create analysis explanation for the result."""
        return {
            "recognizer": self.name,
            "reason": reason,
        }

    def analyze(
        self,
        text: str,
        entities: List[str] = None,
        nlp_artifacts: Dict = None
    ) -> List[RecognizerResult]:
        """
        Analyze text for API keys and secrets.

        Args:
            text: Text to analyze
            entities: Entity types to detect (defaults to API_KEY)
            nlp_artifacts: NLP artifacts (not used)

        Returns:
            List of RecognizerResult for detected API keys
        """
        results = []

        if not text:
            return results

        # Check each pattern
        for name, pattern, base_confidence, description in self.compiled_patterns:
            try:
                for match in pattern.finditer(text):
                    # Get the captured group or full match
                    if match.groups():
                        secret = match.group(1)
                        start = match.start(1)
                        end = match.end(1)
                    else:
                        secret = match.group()
                        start = match.start()
                        end = match.end()

                    confidence = base_confidence

                    # Boost confidence if keyword context is present (for lower confidence patterns)
                    if self.enable_keyword_boost and confidence < 0.85:
                        if self._has_keyword_context(text, start, end):
                            confidence = min(0.95, confidence + 0.15)

                    if confidence >= self.min_confidence:
                        results.append(RecognizerResult(
                            entity_type=self.ENTITY_TYPE,
                            start=start,
                            end=end,
                            score=confidence,
                            analysis_explanation=self._create_explanation(
                                f"{description} ({name})"
                            ),
                        ))
            except Exception as e:
                # Log pattern processing error without potentially sensitive pattern names
                logger.warning(f"Error processing API key detection pattern: {type(e).__name__}")

        # Add entropy-based detection
        if self.enable_entropy_detection:
            entropy_results = self._detect_high_entropy_strings(text)
            results.extend(entropy_results)

        # Remove duplicates and overlapping results (keep highest confidence)
        results = self._remove_overlapping(results)

        return results

    def _remove_overlapping(self, results: List[RecognizerResult]) -> List[RecognizerResult]:
        """
        Remove overlapping results, keeping the highest confidence one.
        """
        if not results:
            return results

        # Sort by start position, then by confidence (descending)
        sorted_results = sorted(results, key=lambda x: (x.start, -x.score))

        filtered = []
        last_end = -1

        for result in sorted_results:
            if result.start >= last_end:
                filtered.append(result)
                last_end = result.end
            elif result.score > filtered[-1].score:
                # Replace with higher confidence
                filtered[-1] = result
                last_end = result.end

        return filtered


# Export for use in main.py
__all__ = ['APIKeyRecognizer', 'API_KEY_PATTERNS', 'API_KEY_KEYWORDS', 'calculate_entropy']
