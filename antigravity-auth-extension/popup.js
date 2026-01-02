// Antigravity Auth Chrome Extension
// OAuth Configuration
const CONFIG = {
    clientId: '1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com',
    clientSecret: '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',
    scopes: [
        'https://www.googleapis.com/auth/cloud-platform',
        'https://www.googleapis.com/auth/userinfo.email',
        'https://www.googleapis.com/auth/userinfo.profile',
        'https://www.googleapis.com/auth/cclog',
        'https://www.googleapis.com/auth/experimentsandconfigs'
    ],
    tokenEndpoint: 'https://oauth2.googleapis.com/token',
    userInfoEndpoint: 'https://www.googleapis.com/oauth2/v1/userinfo?alt=json',
    projectIdEndpoint: 'https://cloudresourcemanager.googleapis.com/v1/projects'
};

// State
let authData = null;

// DOM Elements
const loginSection = document.getElementById('login-section');
const loadingSection = document.getElementById('loading-section');
const successSection = document.getElementById('success-section');
const errorSection = document.getElementById('error-section');
const loginBtn = document.getElementById('login-btn');
const downloadBtn = document.getElementById('download-btn');
const logoutBtn = document.getElementById('logout-btn');
const retryBtn = document.getElementById('retry-btn');
const userEmailEl = document.getElementById('user-email');
const errorMessageEl = document.getElementById('error-message');

// Event Listeners
loginBtn.addEventListener('click', startAuth);
downloadBtn.addEventListener('click', downloadAuthJson);
logoutBtn.addEventListener('click', resetState);
retryBtn.addEventListener('click', resetState);

// Show/Hide UI Sections
function showSection(section) {
    [loginSection, loadingSection, successSection, errorSection].forEach(s => s.classList.add('hidden'));
    section.classList.remove('hidden');
}

// Start OAuth Flow
async function startAuth() {
    showSection(loadingSection);

    try {
        // Build auth URL
        const redirectUri = chrome.identity.getRedirectURL();
        const authUrl = new URL('https://accounts.google.com/o/oauth2/v2/auth');
        authUrl.searchParams.set('client_id', CONFIG.clientId);
        authUrl.searchParams.set('redirect_uri', redirectUri);
        authUrl.searchParams.set('response_type', 'code');
        authUrl.searchParams.set('scope', CONFIG.scopes.join(' '));
        authUrl.searchParams.set('access_type', 'offline');
        authUrl.searchParams.set('prompt', 'consent');

        // Launch web auth flow
        const responseUrl = await new Promise((resolve, reject) => {
            chrome.identity.launchWebAuthFlow(
                { url: authUrl.toString(), interactive: true },
                (responseUrl) => {
                    if (chrome.runtime.lastError) {
                        reject(new Error(chrome.runtime.lastError.message));
                    } else if (!responseUrl) {
                        reject(new Error('No response URL received'));
                    } else {
                        resolve(responseUrl);
                    }
                }
            );
        });

        // Extract auth code
        const url = new URL(responseUrl);
        const authCode = url.searchParams.get('code');
        if (!authCode) {
            throw new Error('No authorization code received');
        }

        // Exchange code for tokens
        const tokenData = await exchangeCodeForTokens(authCode, redirectUri);

        // Fetch user info
        const userInfo = await fetchUserInfo(tokenData.access_token);

        // Try to fetch project ID (optional)
        let projectId = '';
        try {
            projectId = await fetchProjectId(tokenData.access_token);
        } catch (e) {
            console.warn('Could not fetch project ID:', e);
        }

        // Build auth data
        const now = new Date();
        authData = {
            type: 'antigravity',
            access_token: tokenData.access_token,
            refresh_token: tokenData.refresh_token,
            expires_in: tokenData.expires_in,
            timestamp: now.getTime(),
            expired: new Date(now.getTime() + tokenData.expires_in * 1000).toISOString(),
            email: userInfo.email || ''
        };

        if (projectId) {
            authData.project_id = projectId;
        }

        // Show success
        userEmailEl.textContent = authData.email || 'Unknown user';
        showSection(successSection);

    } catch (error) {
        console.error('Auth error:', error);
        errorMessageEl.textContent = error.message || 'Unknown error occurred';
        showSection(errorSection);
    }
}

// Exchange auth code for tokens
async function exchangeCodeForTokens(code, redirectUri) {
    const response = await fetch(CONFIG.tokenEndpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({
            code: code,
            client_id: CONFIG.clientId,
            client_secret: CONFIG.clientSecret,
            redirect_uri: redirectUri,
            grant_type: 'authorization_code'
        })
    });

    if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.error_description || `Token exchange failed: ${response.status}`);
    }

    return response.json();
}

// Fetch user info
async function fetchUserInfo(accessToken) {
    const response = await fetch(CONFIG.userInfoEndpoint, {
        headers: { 'Authorization': `Bearer ${accessToken}` }
    });

    if (!response.ok) {
        throw new Error(`Failed to fetch user info: ${response.status}`);
    }

    return response.json();
}

// Fetch project ID (optional)
async function fetchProjectId(accessToken) {
    const response = await fetch(CONFIG.projectIdEndpoint, {
        headers: { 'Authorization': `Bearer ${accessToken}` }
    });

    if (!response.ok) {
        return '';
    }

    const data = await response.json();
    if (data.projects && data.projects.length > 0) {
        return data.projects[0].projectId;
    }
    return '';
}

// Download auth JSON
function downloadAuthJson() {
    if (!authData) {
        alert('No auth data available');
        return;
    }

    const email = authData.email || 'unknown';
    const sanitizedEmail = email.replace(/@/g, '_').replace(/\./g, '_');
    const filename = `antigravity-${sanitizedEmail}.json`;

    const blob = new Blob([JSON.stringify(authData, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);

    chrome.downloads.download({
        url: url,
        filename: filename,
        saveAs: true
    }, (downloadId) => {
        if (chrome.runtime.lastError) {
            console.error('Download error:', chrome.runtime.lastError);
            alert('Download failed: ' + chrome.runtime.lastError.message);
        } else {
            console.log('Download started:', downloadId);
        }
        URL.revokeObjectURL(url);
    });
}

// Reset state
function resetState() {
    authData = null;
    showSection(loginSection);
}
