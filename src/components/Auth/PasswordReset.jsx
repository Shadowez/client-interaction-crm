import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { supabase } from "../../lib/supabaseClient";
import BrandMark from "../Common/BrandMark";
import "../../styles/PasswordReset.css";

const PasswordReset = () => {
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [tokenValid, setTokenValid] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    const verifySession = async () => {
      try {
        const {
          data: { session },
          error: sessionError,
        } = await supabase.auth.getSession();

        if (sessionError) throw sessionError;
        if (!session?.user) throw new Error("No recovery session found.");

        setTokenValid(true);
        setMessage("Choose a new password for your account.");
      } catch (err) {
        console.error("Password recovery session validation failed:", err);
        setError("This recovery link is invalid or has expired. Request a new one.");
      }
    };

    verifySession();
  }, []);

  const handlePasswordUpdate = async (event) => {
    event.preventDefault();
    setError("");

    if (newPassword !== confirmPassword) {
      setError("Passwords do not match.");
      return;
    }

    if (newPassword.length < 8) {
      setError("Use at least 8 characters.");
      return;
    }

    setLoading(true);
    try {
      const { error: updateError } = await supabase.auth.updateUser({
        password: newPassword,
      });
      if (updateError) throw updateError;

      await supabase.auth.signOut({ scope: "local" });
      navigate("/login", { replace: true });
    } catch (err) {
      setError(err?.message || "Unable to update the password.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="password-reset-container">
      <div className="password-reset-form">
        <div className="auth-brand"><BrandMark /></div>
        <h2>Set password</h2>
        {message && <div className="message">{message}</div>}
        {error && <div className="error">{error}</div>}
        {tokenValid ? (
          <form onSubmit={handlePasswordUpdate}>
            <div className="form-group">
              <label htmlFor="new-password">New password</label>
              <input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                autoComplete="new-password"
                required
                disabled={loading}
                minLength={8}
              />
            </div>
            <div className="form-group">
              <label htmlFor="confirm-password">Confirm password</label>
              <input
                id="confirm-password"
                type="password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                autoComplete="new-password"
                required
                disabled={loading}
              />
            </div>
            <button type="submit" disabled={loading}>
              {loading ? "Updating…" : "Save password"}
            </button>
          </form>
        ) : (
          <div className="back-to-login">
            <Link to="/forgot-password">Request a new link</Link>
          </div>
        )}
      </div>
    </div>
  );
};

export default PasswordReset;
