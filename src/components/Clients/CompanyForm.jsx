import { useCallback, useEffect, useState } from "react";
import { supabase } from "../../lib/supabaseClient";
import CallForm from "../Calls/CallForm";
import "../../styles/CompanyForm.css";

const CompanyForm = ({ companyId, onClose }) => {
  const [company, setCompany] = useState(null);
  const [calls, setCalls] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showCallForm, setShowCallForm] = useState(false);
  const [error, setError] = useState("");

  const fetchCompanyData = useCallback(async () => {
    const { data, error: queryError } = await supabase
      .from("clients")
      .select("*")
      .eq("id", companyId)
      .single();
    if (queryError) throw queryError;
    setCompany(data);
  }, [companyId]);

  const fetchCompanyCalls = useCallback(async () => {
    const { data, error: queryError } = await supabase
      .from("calls")
      .select("*")
      .eq("client_id", companyId)
      .order("call_date", { ascending: false });
    if (queryError) throw queryError;
    setCalls(data || []);
  }, [companyId]);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      await Promise.all([fetchCompanyData(), fetchCompanyCalls()]);
    } catch (err) {
      console.error("Unable to load company details:", err);
      setError("Unable to load company details.");
    } finally {
      setLoading(false);
    }
  }, [fetchCompanyCalls, fetchCompanyData]);

  useEffect(() => {
    if (companyId) load();
  }, [companyId, load]);

  if (loading) {
    return (
      <div className="company-form-overlay">
        <div className="company-form-container"><div className="loading">Loading…</div></div>
      </div>
    );
  }

  if (!company || error) {
    return (
      <div className="company-form-overlay">
        <div className="company-form-container">
          <button className="floating-close" onClick={onClose} aria-label="Close">✕</button>
          <div>{error || "Company not found."}</div>
        </div>
      </div>
    );
  }

  return (
    <div className="company-form-overlay">
      <div className="company-form-container">
        <button className="floating-close" onClick={onClose} aria-label="Close">✕</button>
        <h2>Company details</h2>
        <div className="company-details">
          <div className="detail-row"><span className="detail-label">Company:</span><span className="detail-value">{company.name}</span></div>
          <div className="detail-row"><span className="detail-label">Legal form:</span><span className="detail-value">{company.form || "—"}</span></div>
          <div className="detail-row"><span className="detail-label">Tax ID:</span><span className="detail-value">{company.inn || "—"}</span></div>
          <div className="detail-row"><span className="detail-label">Contact person:</span><span className="detail-value">{company.contact_person || "—"}</span></div>
          <div className="detail-row"><span className="detail-label">Address:</span><span className="detail-value">{company.address || "—"}</span></div>
          <div className="detail-row">
            <span className="detail-label">Website:</span>
            <span className="detail-value">
              {company.website ? <a href={company.website} target="_blank" rel="noopener noreferrer">{company.website}</a> : "—"}
            </span>
          </div>
          <div className="detail-row"><span className="detail-label">Phone:</span><span className="detail-value">{company.phone || "—"}</span></div>
        </div>

        <h3>Interaction history</h3>
        <div className="call-actions">
          <button className="add-call-btn" onClick={() => setShowCallForm(true)}>Add interaction</button>
        </div>

        {calls.length === 0 ? (
          <p>No interactions yet.</p>
        ) : (
          <table className="calls-table">
            <thead><tr><th>Date</th><th>Created by</th><th>Description</th></tr></thead>
            <tbody>
              {calls.map((call) => (
                <tr key={call.id}>
                  <td>{new Date(`${call.call_date}T00:00:00`).toLocaleDateString()}</td>
                  <td>{call.created_by_email || "Team member"}</td>
                  <td>{call.description}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {showCallForm && (
          <CallForm
            clientId={companyId}
            onSave={() => {
              setShowCallForm(false);
              fetchCompanyCalls().catch((err) => {
                console.error("Unable to refresh interactions:", err);
                setError("Unable to refresh interactions.");
              });
            }}
            onCancel={() => setShowCallForm(false)}
          />
        )}
      </div>
    </div>
  );
};

export default CompanyForm;
