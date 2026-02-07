import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';

interface LayoutProps {
  children: React.ReactNode;
}

// Runtime config from window (injected via config.js)
interface AppConfig {
  banner?: {
    enabled: boolean;
    message: string;
    type: 'warning' | 'info';
  };
  version?: string;
}

declare global {
  interface Window {
    APP_CONFIG?: AppConfig;
  }
}

export function Layout({ children }: LayoutProps) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const banner = window.APP_CONFIG?.banner;

  const handleLogout = async () => {
    await logout();
    navigate('/auth');
  };

  return (
    <div className="min-h-screen flex flex-col bg-slate-50">
      {/* Environment Banner */}
      {banner?.enabled && (
        <div className={`px-4 py-2 text-center text-sm font-medium ${
          banner.type === 'warning' 
            ? 'bg-amber-500 text-white' 
            : 'bg-blue-500 text-white'
        }`}>
          {banner.message}
        </div>
      )}

      {/* Navigation Bar - bg-blue-700 is fallback for browsers that don't support gradients */}
      <nav className="bg-blue-700 bg-gradient-to-r from-blue-600 to-blue-800 text-white py-3 shadow-md sticky top-0 z-50">
        <div className="max-w-6xl mx-auto px-3 sm:px-6 flex justify-between items-center">
          <Link to="/orders" className="flex items-center gap-2 sm:gap-3 text-xl sm:text-2xl font-bold tracking-tight shrink-0">
            <img src="/images/nav-trans.png" alt="Fish Fry Logo" className="h-8 sm:h-10 w-auto" />
            <span className="hidden md:inline">Fish Fry Orders</span>
          </Link>
          <div className="flex gap-1 sm:gap-2 items-center">
            <Link
              to="/orders/new"
              className="px-2 sm:px-4 py-2 rounded-md text-sm sm:text-base font-medium hover:bg-white/15 transition-all hover:-translate-y-0.5 whitespace-nowrap"
            >
              New Order
            </Link>
            <Link
              to="/orders"
              className="px-2 sm:px-4 py-2 rounded-md text-sm sm:text-base font-medium hover:bg-white/15 transition-all hover:-translate-y-0.5"
            >
              Orders
            </Link>
            {user?.role === 'admin' && (
              <Link
                to="/admin"
                className="px-2 sm:px-4 py-2 rounded-md text-sm sm:text-base font-medium hover:bg-white/15 transition-all hover:-translate-y-0.5"
              >
                Admin
              </Link>
            )}
            <button
              onClick={handleLogout}
              className="px-2 sm:px-4 py-2 rounded-md text-sm sm:text-base font-medium hover:bg-white/15 transition-all hover:-translate-y-0.5"
            >
              Logout
            </button>
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <main className="flex-1 max-w-6xl w-full mx-auto my-8 px-6">
        {children}
      </main>

      {/* Footer */}
      <footer className="mt-auto py-3 text-center bg-white border-t border-slate-200 text-xs text-slate-500">
        <span className="font-mono">Fish Fry Orders</span>
        {window.APP_CONFIG?.version && (
          <span className="font-mono text-slate-400 ml-2">({window.APP_CONFIG.version})</span>
        )}
      </footer>
    </div>
  );
}
