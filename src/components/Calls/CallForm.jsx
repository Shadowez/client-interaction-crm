import { useEffect, useState } from "react";
import { supabase } from "../../lib/supabaseClient";
import "../../styles/CallForm.css";

const CallForm = ({ call, onSave, onCancel, clientId: preselectedClientId }) => {
  const [callDate, setCallDate] = useState(call?.call_date || new Date().toISOString().split("T")[0]);
  const [description, setDescription] = useState(call?.description || "");
  const [clientId, setClientId] = useState(call?.client_id || preselectedClientId || "");
  const [clients, setClients] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const fetchClients = async () => {
      try {
        const { data, error: queryError } = await supabase
          .from("clients")
          .select("id, name")
          .order("name");
        if (queryError) throw queryError;
        setClients(data || []);
      } catch (err) {
        console.error("Unable to load companies:", err);
        setError("Unable to load the company list.");
      }
    };
    fetchClients();
  }, []);

  const handleSubmit = async (event) => {
    event.preventDefault();
    setError("");

    const cleanDescription = description.trim();
    if (!callDate || !cleanDescription || !clientId) {
      setError("Date, company, and description are required.");
      return;
    }

    const interactionData = {
      call_date: callDate,
      description: cleanDescription,
      client_id: Number(clientId),
    };

    setLoading(true);
    try {
      const query = call
        ? supabase.from("calls").update(interactionData).eq("id", call.id)
        : supabase.from("calls").insert(interactionData);
      const { error: saveError } = await query;
      if (saveError) throw saveError;
      onSave();
    } catch (err) {
      console.error("Unable to save interaction:", err);
      setError(err?.message || "Unable to save the interaction.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="form-overlay">
      <div className="form-container">
        <h2>{call ? "Edit interaction" : "Add interaction"}</h2>
        {error && <div className="form-error">{error}</div>}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="interaction-date">Date *</label>
            <input
              id="interaction-date"
              type="date"
              value={callDate}
              onChange={(event) => setCallDate(event.target.value)}
              required
              disabled={loading}
            />
          </div>
          <div className="form-group">
            <label htmlFor="interaction-company">Company *</label>
            {preselectedClientId ? (
              <div className="preselected-client">
                {clients.find((client) => client.id === Number(preselectedClientId))?.name || "Selected company"}
              </div>
            ) : (
              <select
                id="interaction-company"
                value={clientId}
                onChange={(event) => setClientId(event.target.value)}
                required
                disabled={loading}
              >
                <option value="">Select a company</option>
                {clients.map((client) => (
                  <option key={client.id} value={client.id}>{client.name}</option>
                ))}
              </select>
            )}
          </div>
          <div className="form-group">
            <label htmlFor="interaction-description">Description *</label>
            <textarea
              id="interaction-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              required
              maxLength={5000}
              rows="5"
              disabled={loading}
            />
          </div>
          <div className="form-buttons">
            <button type="submit" disabled={loading}>{loading ? "Saving…" : "Save"}</button>
            <button type="button" onClick={onCancel} disabled={loading}>Cancel</button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default CallForm;
