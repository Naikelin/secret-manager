import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

// Protected routes that require authentication
const protectedRoutes = ['/dashboard', '/secrets', '/audit', '/drift'];

// Public routes that don't require authentication
const publicRoutes = ['/auth/login', '/auth/callback'];

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  
  // Check if the route is protected
  const isProtectedRoute = protectedRoutes.some(route => 
    pathname.startsWith(route)
  );
  
  const isPublicRoute = publicRoutes.some(route => 
    pathname.startsWith(route)
  );

  // For protected routes, check authentication
  if (isProtectedRoute) {
    // In client-side auth with localStorage, we can't check the token here
    // The actual check will be done in the page component
    // This middleware primarily serves to document protected routes
    return NextResponse.next();
  }

  // Allow public routes
  if (isPublicRoute) {
    return NextResponse.next();
  }

  // Default: allow the request
  return NextResponse.next();
}

// Configure which routes should run middleware
export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     * - public folder
     */
    '/((?!_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp)$).*)',
  ],
};
