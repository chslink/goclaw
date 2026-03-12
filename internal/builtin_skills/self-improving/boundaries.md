# Security Boundaries

## Never Store

These categories must NEVER be written to any self-improving file:

| Category | Examples |
|----------|----------|
| **Credentials** | Passwords, API keys, tokens, SSH keys, certificates |
| **Financial** | Account numbers, card numbers, balances, transactions |
| **Medical** | Health conditions, medications, diagnoses |
| **Biometric** | Physical descriptions tied to identity |
| **Third parties** | Other people's preferences, habits, or personal info |
| **Location patterns** | Home address, work address, daily routes |
| **Access patterns** | Login times, security questions, recovery codes |

## Store with Caution

These may be stored but require care:

| Category | Rule |
|----------|------|
| **Work context** | Project names OK; confidential details NO |
| **Emotional states** | General preferences OK; vulnerabilities NO |
| **Relationships** | Professional context OK; personal details NO |
| **Schedules** | Time zone and general availability OK; exact calendar NO |

## Transparency Requirements

1. **Audit on demand** — When asked "what do you know about me?", provide complete answer
2. **Source tracking** — Every memory entry should note when and why it was created
3. **Explain actions** — When applying a learned pattern, briefly mention it: "Based on your preference for X..."
4. **No hidden state** — Everything the agent "knows" must be in the self-improving files
5. **Deletion verification** — When asked to forget something, confirm deletion and verify

## Kill Switch

When the user says "forget everything" or equivalent:

1. **Export** — Offer to export current memory before deletion
2. **Wipe** — Delete all files in self-improving directory
3. **Confirm** — Show the user the directory is empty
4. **No ghost patterns** — Do not retain any learned behaviors after wipe
