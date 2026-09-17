// unofi Admin Dashboard — JavaScript
// Uses the existing backend APIs (POST /api/admin/login, GET /api/admin/...)
// and the existing HttpOnly session cookie for authentication.

(function () {
  // --- Helpers ---

  function $(sel) { return document.querySelector(sel); }
  function qsa(sel) { return document.querySelectorAll(sel); }

  function showAdminError(msg) {
    const el = $('#admin-error');
    if (el) {
      el.textContent = msg;
      el.style.display = 'block';
    }
  }

  function hideAdminError() {
    const el = $('#admin-error');
    if (el) el.style.display = 'none';
  }

  function api(method, path, body) {
    const url = `/api${path}`;
    const opts = {
      method,
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
    };
    if (body !== undefined) {
      opts.body = JSON.stringify(body);
    }
    return fetch(url, opts);
  }

  function apiJSON(method, path, body) {
    return api(method, path, body).then(r => {
      if (!r.ok) {
        if (r.status === 401 || r.status === 403) {
          // Unauthorized — force re-login
          showScreen('login');
          $('admin-login-screen').classList.add('show');
          $('admin-dashboard').style.display = 'none';
          // Clear any stale session cookie attempt
          document.cookie = 'session=; Max-Age=0; Path=/;';
        }
        return r.text().then(txt => { throw new Error(txt || 'API error'); });
      }
      return r.json();
    });
  }

  function showScreen(name) {
    ['login', 'dashboard'].forEach(s => {
      const sEl = $(`#admin-${s}-screen`);
      if (sEl) sEl.style.display = (s === name) ? 'block' : 'none';
    });
  }

  // --- Check login status on page load ---

  function checkAuth() {
    apiJSON('GET', '/api/admin/check').then(data => {
      if (data && data.status === 'ok') {
        // Authenticated — show dashboard, hide login
        $('admin-login-screen').classList.remove('show');
        $('admin-dashboard').style.display = 'block';
        $('admin-username').textContent = 'Administrator';
        $('admin-session-status').textContent = 'Session active';
        // Initial fetch of dashboard data
        fetchOverview();
      } else {
        // Not authenticated — show login screen
        $('admin-login-screen').classList.add('show');
        $('admin-dashboard').style.display = 'none';
      }
    }).catch(() => {
      $('admin-login-screen').classList.add('show');
      $('admin-dashboard').style.display = 'none';
    });
  }

  // --- Admin Login ---

$('admin-signin').addEventListener('click', () => {
    const pw = $('admin-password').value.trim();
    if (pw.length === 0) {
      $('admin-password').setCustomValidity('Password required');
      return;
    }
    hideAdminError();
    apiJSON('POST', '/api/admin/login', { password: pw }).then(data => {
      // 'init' means first password ever — successful initialization
      // 'ok' means password verified, existing admin session
      if (data && (data.status === 'init' || data.status === 'ok')) {
        $('admin-password').value = '';
        // Hide login, show dashboard
        $('admin-login-screen').classList.remove('show');
        $('admin-dashboard').style.display = 'block';
        $('admin-username').textContent = 'Administrator';
        $('admin-session-status').textContent = 'Session active';
        // Initial fetch of dashboard data
        fetchOverview();
      } else {
        showAdminError('Invalid administrator password.');
      }
    }).catch(() => {
      showAdminError('Unable to connect to UNOFI server. Try again.');
    });
  })();

  $('admin-logout').addEventListener('click', () => {
    apiJSON('GET', '/admin/logout').then(() => {
      // Clear cookie and show login
      document.cookie = 'session=; Max-Age=0; Path=/;';
      $('admin-login-screen').classList.add('show');
      $('admin-dashboard').style.display = 'none';
      Portal.toast('Logged out', true);
    }).catch(err => {
      Portal.toast('Logout error: ' + err.message, false);
    });
  });

  // --- Dashboard: Overview fetch ---

  function fetchOverview() {
    // Fetch all needed data concurrently
    Promise.all([
      apiJSON('GET', '/admin/devices?filter=active').then(r => {
        const total = (r || []).length;
        const activeEl = $('overview-users-total');
        if (activeEl) activeEl.textContent = total > 0 ? total : '0';
        const activeTextEl = $('overview-users-text');
        if (activeTextEl) activeTextEl.textContent = total > 0 ? `${total} user${total !== 1 ? 's' : ''} online` : 'no users yet';
      }).catch(() => { if ($('overview-users-total')) $('overview-users-total').textContent = 'err'; }),

      apiJSON('GET', '/api/admin/check').then(r => {
        // Session already checked above, but use for active sessions count
      }).catch(() => {}),

      apiJSON('GET', '/admin/earnings').then(r => {
        // Earnings summary card - not fully implemented in dashboard yet
      }).catch(() => {}),

      apiJSON('GET', '/admin/system').then(r => {
        const uptimeEl = $('overview-system-uptime');
        const cpuEl = $('overview-system-text');
        if (uptimeEl && r && r.uptime) {
          const mins = Math.floor(r.uptime / 60);
          uptimeEl.textContent = mins ? `${mins} min` : '0 min';
        }
        if (cpuEl) cpuEl.textContent = r && r.cpu_temp !== undefined ? `${r.cpu_temp}°C` : 'N/A';
      }).catch(() => {}),

      apiJSON('GET', '/admin/txn?limit=5').then(r => {
        const txnEl = $('overview-users-text'); // reuse
      }).catch(() => {}),
    ]).then(() => {
      // Also fetch active sessions count
      apiJSON('GET', '/admin/devices?filter=active').then(devs => {
        const activeEl = $('overview-sessions-active');
        if (activeEl) activeEl.textContent = devs && devs.length ? devs.length : '0';
        if (devs && devs.length > 0) {
          const dev = devs[0];
          const duStr = du ? `${dev.MBUsed > dev.MBLimit ? dev.MBLimit : dev.MBUsed}MB/${dev.MBLimit}MB` : '0MB';
          // Show simple text
          if ($('overview-sessions-text')) $('overview-sessions-text').textContent = `${devs.length} active session${devs.length !== 1 ? 's' : ''}`;
        } else {
          if ($('overview-sessions-text')) $('overview-sessions-text').textContent = 'no active sessions';
        }
      }).catch(() => {
        if ($('overview-sessions-active')) $('overview-sessions-active').textContent = '?';
      });
    });
  }

  // --- Page navigation ---

  const adminPages = {
    'dashboard': 'dashboard-home',
    'users': 'users-page',
    'sessions': 'sessions-page',
    'system': 'system-page',
    'maintenance': 'maintenance-page',
  };

  function navigateTo(pageKey) {
    // Hide all sections
    Object.values(adminPages).forEach(id => {
      const el = $(id);
      if (el) el.style.display = 'none';
    });
    // Remove active class from all nav items
    qsa('#admin-nav .admin-nav-item').forEach(li => li.classList.remove('active'));
    // Show selected section
    const targetId = adminPages[pageKey];
    if (targetId) {
      $(targetId).style.display = 'block';
      $(`#admin-nav .admin-nav-item[data-page="${pageKey}"]`).classList.add('active');
    }
    // Update URL hash for bookmarking
    window.location.hash = pageKey;
  }

  // --- Init: listen for hash changes, navigate on load ---

  window.addEventListener('hashchange', () => {
    const hash = location.hash.slice(1) || 'dashboard';
    navigateTo(hash);
  });

  // On load, check auth then navigate
  checkAuth();

  // Handle hash on first load (before auth check completes)
  const initialHash = location.hash.slice(1) || 'dashboard';
  setTimeout(() => {
    if ($('admin-dashboard').style.display !== 'none') {
      navigateTo(initialHash);
    }
  }, 100);

  // --- Sidebar mobile toggle ---

  const sidebar = $('#admin-sidebar');
  const main = $('#admin-main');
  let sidebarCollapsed = false;

  // Already handled by CSS/mediaqueries for now
})();