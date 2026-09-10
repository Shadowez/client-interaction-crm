import { useEffect, useState } from "react";
import {
  BrowserRouter as Router,
  Navigate,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import { supabase, supabaseConfigurationError } from "./lib/supabaseClient";
import Login from "./components/Auth/Login";
import ClientList from "./components/Clients/ClientList";
import CallJournal from "./components/Calls/CallJournal";
import Navigation from "./components/Common/Navigation";
import PasswordReset from "./components/Auth/PasswordReset";
import ForgotPassword from "./components/Auth/ForgotPassword";
import branding from "./config/branding.json";
import "./App.css";

const routerBase =
  import.meta.env.BASE_URL === "/"
    ? "/"
    : import.meta.env.BASE_URL.replace(/\/$/, "");

function AppContent() {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const location = useLocation();

  useEffect(() => {
    document.title = branding.organizationName?.trim()
      ? `${branding.organizationName} — ${branding.productName}`
      : branding.productName;
  }, []);

  useEffect(() => {
    const getSession = async () => {
      try {
        const {
          data: { session },
          error,
        } = await supabase.auth.getSession();

        if (error) throw error;
        setUser(session?.user ?? null);
      } catch (error) {
        console.error("Unable to restore authentication session:", error);
      } finally {
        setLoading(false);
      }
    };

    getSession();

    const {
      data: { subscription },
    } = supabase.auth.onAuthStateChange((_event, session) => {
      setUser(session?.user ?? null);
      setLoading(false);
    });

    return () => subscription?.unsubscribe();
  }, []);

  const ProtectedRoute = ({ children }) =>
    user ? children : <Navigate to="/login" replace />;

  if (supabaseConfigurationError) {
    return (
      <div className="configuration-error-page">
        <div className="configuration-error-card">
          <h1>Configuration required</h1>
          <p>{supabaseConfigurationError}</p>
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="global-loading-overlay">
        <div className="global-loading">Loading…</div>
      </div>
    );
  }

  const hideNavigation = ["/update-password", "/forgot-password"].includes(
    location.pathname
  );

  return (
    <div className="App">
      {user && !hideNavigation && <Navigation />}
      <Routes>
        <Route
          path="/login"
          element={
            user ? <Navigate to="/clients" replace /> : <Login onLogin={setUser} />
          }
        />
        <Route
          path="/clients"
          element={
            <ProtectedRoute>
              <ClientList user={user} />
            </ProtectedRoute>
          }
        />
        <Route
          path="/interactions"
          element={
            <ProtectedRoute>
              <CallJournal user={user} />
            </ProtectedRoute>
          }
        />
        <Route path="/calls" element={<Navigate to="/interactions" replace />} />
        <Route path="/update-password" element={<PasswordReset />} />
        <Route path="/forgot-password" element={<ForgotPassword />} />
        <Route
          path="*"
          element={<Navigate to={user ? "/clients" : "/login"} replace />}
        />
      </Routes>
    </div>
  );
}

function App() {
  return (
    <Router basename={routerBase}>
      <AppContent />
    </Router>
  );
}

export default App;
