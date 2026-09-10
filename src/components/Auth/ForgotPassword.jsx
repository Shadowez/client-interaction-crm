import { useState } from "react";
import { Link } from "react-router-dom";
import { supabase } from "../../lib/supabaseClient";
import BrandMark from "../Common/BrandMark";
import "../../styles/ForgotPassword.css";

const getPasswordResetUrl = () => {
  const basePath = import.meta.env.BASE_URL.endsWith("/")
    ? import.meta.env.BASE_URL
    : `${import.meta.env.BASE_URL}/`;
  return new URL(`${basePath}update-password`, window.location.origin).toString();
};

const ForgotPassword = () => {
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleResetRequest = async (event) => {
    event.preventDefault();
    setError("");
    setMessage("");
    setLoading(true);

    try {
      const { error: resetError } = await supabase.auth.resetPasswordForEmail(
        email.trim(),
        { redirectTo: getPasswordResetUrl() }
      );

      if (resetError) throw resetError;
      setMessage("Check your email for a password reset link.");
    } catch (err) {
      setError(err?.message || "Unable to send a reset link.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="forgot-password-container">
      <div className="forgot-password-form">
        <div className="auth-brand"><BrandMark /></div>
        <h2>Reset password</h2>
        <p>Enter your email address and we will send you a reset link.</p>
        {message && <div className="message">{message}</div>}
        {error && <div className="error">{error}</div>}
        <form onSubmit={handleResetRequest}>
          <div className="form-group">
            <label htmlFor="reset-email">Email</label>
            <input
              id="reset-email"
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              autoComplete="email"
              required
              disabled={loading}
            />
          </div>
          <button type="submit" disabled={loading}>
            {loading ? "Sending…" : "Send reset link"}
          </button>
        </form>
        <div className="back-to-login">
          <Link to="/login">Back to sign in</Link>
        </div>
      </div>
    </div>
  );
};

export default ForgotPassword;
