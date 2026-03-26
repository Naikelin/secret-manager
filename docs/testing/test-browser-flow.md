# Browser Test - Auth Flow End-to-End

## Test Steps

### 1. Open Login Page
- Navigate to: `http://localhost:3000/auth/login`
- **Expected**: See "Secret Manager" title, email input pre-filled with "dev@example.com", "Login with OAuth" button

### 2. Click Login Button
- Click "Login with OAuth" button
- **Expected**: 
  - Button shows "Logging in..." temporarily
  - Page redirects to backend mock-callback
  - Automatically redirects to frontend callback handler
  - Finally lands on dashboard

### 3. Verify Dashboard
- **Expected**: 
  - URL: `http://localhost:3000/` or `http://localhost:3000/dashboard`
  - Page shows user information
  - Check browser console (F12) for: `Mock auth successful: dev@example.com`

### 4. Verify localStorage
- Open browser DevTools (F12) → Application tab → Local Storage → `http://localhost:3000`
- **Expected**:
  - `auth_token`: JWT string (3 parts separated by dots)
  - `user`: JSON object with `{"id":"...","email":"dev@example.com","name":"Developer User","groups":["developers"]}`

### 5. Test with Admin User
- Go back to login: `http://localhost:3000/auth/login`
- Change email to: `admin@example.com`
- Click "Login with OAuth"
- **Expected**:
  - Login succeeds
  - Dashboard shows admin user info
  - localStorage `user` object has `groups: ["admins"]`

## Troubleshooting

### Issue: Button doesn't do anything
- **Check**: Browser console for errors (F12 → Console tab)
- **Check**: Backend logs: `docker-compose logs backend --tail 50`
- **Possible cause**: CORS error, backend not running, email validation failing

### Issue: Redirect to login with error
- **Check**: URL query params (e.g., `?error=invalid_token`)
- **Check**: Browser console for error messages
- **Possible cause**: JWT decode failed, backend returned invalid token

### Issue: Stuck on "Completing authentication..."
- **Check**: Browser console for errors
- **Possible cause**: Callback handler crash, localStorage not writable

## Success Criteria

✅ User can login with `dev@example.com`  
✅ User can login with `admin@example.com`  
✅ JWT token stored in localStorage  
✅ User info stored in localStorage  
✅ Dashboard displays user information  
✅ No console errors  
✅ Entire flow completes in < 3 seconds  
