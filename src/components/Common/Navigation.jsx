import { NavLink, useNavigate } from "react-router-dom";
import { supabase } from "../../lib/supabaseClient";
import BrandMark from "./BrandMark";
import "../../styles/Navigation.css";

const Navigation = () => {
  const navigate = useNavigate();

  const handleLogout = async () => {
    try {
      const { error } = await supabase.auth.signOut({ scope: "local" });
      if (error) throw error;
      navigate("/login", { replace: true });
    } catch (err) {
      console.error("Unable to sign out:", err);
    }
  };

  return (
    <nav className="navigation">
      <div className="nav-content">
        <div className="nav-brand">
          <BrandMark compact />
          <div className="nav-links">
            <NavLink
              to="/clients"
              className={({ isActive }) =>
                `nav-link ${isActive ? "nav-link-active" : ""}`
              }
            >
              Companies
            </NavLink>
            <NavLink
              to="/interactions"
              className={({ isActive }) =>
                `nav-link ${isActive ? "nav-link-active" : ""}`
              }
            >
              Interactions
            </NavLink>
          </div>
        </div>
        <div className="logout-container">
          <button onClick={handleLogout} className="logout-btn">
            Sign out
          </button>
        </div>
      </div>
    </nav>
  );
};

export default Navigation;
