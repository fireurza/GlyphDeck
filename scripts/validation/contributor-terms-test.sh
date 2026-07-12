#!/usr/bin/env bash
# Smoke tests for the Contributor Terms workflow grep logic.
# Run: bash scripts/validation/contributor-terms-test.sh
set -e

PASS=0
FAIL=0

check() {
  local desc="$1"
  local body="$2"
  local want="$3"
  local accepted
  accepted=$(echo "$body" | grep -c '\- \[[xX]\] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution.' || true)

  if [ "$accepted" -ge 1 ] && [ "$want" = "accept" ]; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  elif [ "$accepted" -eq 0 ] && [ "$want" = "reject" ]; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (accepted=$accepted want=$want)"
    FAIL=$((FAIL + 1))
  fi
}

# ---- Normal cases ----
check "normal checked box" \
  "- [x] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution." \
  "accept"

check "uppercase X checked" \
  "- [X] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution." \
  "accept"

check "unchecked box" \
  "- [ ] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution." \
  "reject"

check "missing statement completely" \
  "## Summary\nSome text here." \
  "reject"

# ---- Multiline content ----
check "checked box in multiline body" \
  "## Summary\nSome text\n- [x] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution.\nMore text" \
  "accept"

# ---- Shell metacharacters ----
check "quotes in body" \
  "- [x] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution.\nSome text with \"double quotes\" and 'single quotes'" \
  "accept"

check "backticks in body" \
  "- [x] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution.\nRun \`echo hello\`" \
  "accept"

check "command substitution attempt" \
  "- [x] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution.\n\$(whoami)" \
  "accept"

check "shell metacharacters" \
  "- [x] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution.\n| & ; < > ( ) { }" \
  "accept"

check "newlines everywhere" \
  "  \n- [x] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution.\n  \n" \
  "accept"

# ---- Malicious injection attempts ----
check "malicious text attempting shell execution" \
  "- [x] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution.\nalert('xss')" \
  "accept"

check "backtick injection" \
  "- [x] I have read and agree to the GlyphDeck Contributor Terms and confirm that I am authorized to submit this contribution.\n\`cat /etc/passwd\`" \
  "accept"

echo ""
echo "Results: $PASS pass, $FAIL fail"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
