# Security Policy

## 🔒 Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 6.x     | ✅ Yes             |
| 5.x     | ⚠️ Security fixes only |
| < 5.0   | ❌ No              |

## 🚨 Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please follow these steps:

### Do NOT:
- ❌ Open a public GitHub issue
- ❌ Post about it on social media
- ❌ Share details publicly before it's fixed

### Do:
1. **Email us directly** at: `security@astroalpha.dev` (or contact via [Facebook](https://www.facebook.com/lehuyducanh/))
2. **Include:**
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Any suggested fixes (optional)

### What to Expect:
- 📬 **Acknowledgment** within 48 hours
- 🔍 **Initial assessment** within 1 week
- 🛠️ **Fix timeline** communicated based on severity
- 🏆 **Credit** given in release notes (if desired)

## 🛡️ Security Best Practices

When deploying CLIProxy Dashboard:

1. **Always use a strong `secret-key`** in your `config.yaml`
2. **Never expose port 8317** directly to the internet without authentication
3. **Use HTTPS** in production (via reverse proxy like Nginx/Caddy)
4. **Regularly update** to the latest version
5. **Limit access** to the management dashboard to trusted IPs

## 📜 Disclosure Policy

We follow a **90-day disclosure policy**:
- After a vulnerability is reported, we have 90 days to release a fix
- After the fix is released, we will publish a security advisory

Thank you for helping keep CLIProxy Dashboard secure! 🙏
