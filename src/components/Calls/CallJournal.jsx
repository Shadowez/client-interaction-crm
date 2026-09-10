import { useCallback, useEffect, useMemo, useState } from "react";
import { supabase } from "../../lib/supabaseClient";
import { DeleteIcon, EditIcon } from "../Common/ActionIcon";
import CallForm from "./CallForm";
import "../../styles/CallJournal.css";

const CallJournal = ({ user }) => {
  const [calls, setCalls] = useState([]);
  const [showForm, setShowForm] = useState(false);
  const [editingCall, setEditingCall] = useState(null);
  const [filterByUser, setFilterByUser] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const filteredCalls = useMemo(() => {
    if (!filterByUser || !user) return calls;
    return calls.filter((call) => call.created_by === user.id);
  }, [calls, filterByUser, user]);

  const fetchCalls = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const { data, error: queryError } = await supabase
        .from("calls")
        .select("*, clients(name)")
        .order("call_date", { ascending: false });
      if (queryError) throw queryError;
      setCalls(data || []);
    } catch (err) {
      console.error("Unable to load interactions:", err);
      setError("Unable to load interactions. Check your connection and Supabase setup.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchCalls();
  }, [fetchCalls]);

  const handleDelete = async (id) => {
    if (!window.confirm("Delete this interaction?")) return;
    try {
      const { error: deleteError } = await supabase.from("calls").delete().eq("id", id);
      if (deleteError) throw deleteError;
      await fetchCalls();
    } catch (err) {
      console.error("Unable to delete interaction:", err);
      setError("Unable to delete the interaction.");
    }
  };

  const closeForm = () => {
    setEditingCall(null);
    setShowForm(false);
  };

  return (
    <div className="clients-container calls-container">
      <div className="header">
        <h1>Interactions</h1>
        <div className="header-controls">
          <button onClick={() => setShowForm(true)}>Add interaction</button>
          <button
            onClick={() => setFilterByUser((value) => !value)}
            className={filterByUser ? "filter-active" : ""}
          >
            {filterByUser ? "All records" : "My records"}
          </button>
        </div>
      </div>

      {error && <div className="page-error">{error}</div>}

      <div className="filter-status">
        {filterByUser
          ? `Showing interactions you created (${filteredCalls.length})`
          : `Showing all interactions (${filteredCalls.length})`}
      </div>

      {showForm && (
        <CallForm
          call={editingCall}
          onSave={() => {
            closeForm();
            fetchCalls();
          }}
          onCancel={closeForm}
        />
      )}

      <table className="calls-table">
        <thead>
          <tr>
            <th>Date</th>
            <th>Company</th>
            <th>Created by</th>
            <th>Description</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr><td colSpan="5" className="loading-row"><div className="loading-spinner">Loading interactions…</div></td></tr>
          ) : filteredCalls.length === 0 ? (
            <tr><td colSpan="5" className="no-data">{filterByUser ? "You have not created any interactions yet." : "No interactions yet."}</td></tr>
          ) : (
            filteredCalls.map((call) => (
              <tr key={call.id}>
                <td>{new Date(`${call.call_date}T00:00:00`).toLocaleDateString()}</td>
                <td>{call.clients?.name || "—"}</td>
                <td>{call.created_by_email || "Team member"}</td>
                <td>{call.description}</td>
                <td>
                  <button
                    onClick={() => {
                      setEditingCall(call);
                      setShowForm(true);
                    }}
                    className="icon-btn edit-btn"
                    title="Edit interaction"
                    aria-label="Edit interaction"
                  >
                    <EditIcon />
                  </button>
                  <button
                    onClick={() => handleDelete(call.id)}
                    className="icon-btn delete-btn"
                    title="Delete interaction"
                    aria-label="Delete interaction"
                  >
                    <DeleteIcon />
                  </button>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
};

export default CallJournal;
