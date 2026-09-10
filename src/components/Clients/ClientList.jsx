import { useCallback, useEffect, useMemo, useState } from "react";
import { supabase } from "../../lib/supabaseClient";
import ClientForm from "./ClientForm";
import CompanyForm from "./CompanyForm";
import { DeleteIcon, EditIcon } from "../Common/ActionIcon";
import "../../styles/ClientList.css";

const ClientList = ({ user }) => {
  const [clients, setClients] = useState([]);
  const [editingClient, setEditingClient] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [selectedCompanyId, setSelectedCompanyId] = useState(null);
  const [filterByUser, setFilterByUser] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const filteredClients = useMemo(() => {
    if (!filterByUser || !user) return clients;
    return clients.filter((client) => client.created_by === user.id);
  }, [clients, filterByUser, user]);

  const fetchClients = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const { data, error: queryError } = await supabase
        .from("clients")
        .select("*")
        .order("name");

      if (queryError) throw queryError;
      setClients(data || []);
    } catch (err) {
      console.error("Unable to load companies:", err);
      setError("Unable to load companies. Check your connection and Supabase setup.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchClients();
  }, [fetchClients]);

  const handleDelete = async (id) => {
    if (!window.confirm("Delete this company and its interaction history?")) return;

    try {
      const { error: deleteError } = await supabase
        .from("clients")
        .delete()
        .eq("id", id);
      if (deleteError) throw deleteError;
      await fetchClients();
    } catch (err) {
      console.error("Unable to delete company:", err);
      setError("Unable to delete the company.");
    }
  };

  const closeForm = () => {
    setEditingClient(null);
    setShowForm(false);
  };

  return (
    <div className="clients-container">
      <div className="header">
        <h1>Companies</h1>
        <div className="header-controls">
          <button onClick={() => setShowForm(true)}>Add company</button>
          <button
            onClick={() => setFilterByUser((value) => !value)}
            className={filterByUser ? "filter-active" : ""}
          >
            {filterByUser ? "All records" : "My records"}
          </button>
        </div>
      </div>

      {error && <div className="page-error">{error}</div>}

      {showForm && (
        <ClientForm
          client={editingClient}
          onSave={() => {
            closeForm();
            fetchClients();
          }}
          onCancel={closeForm}
        />
      )}

      {selectedCompanyId && (
        <CompanyForm
          companyId={selectedCompanyId}
          onClose={() => setSelectedCompanyId(null)}
        />
      )}

      <div className="filter-status">
        {filterByUser
          ? `Showing companies you created (${filteredClients.length})`
          : `Showing all companies (${filteredClients.length})`}
      </div>

      <table className="clients-table">
        <thead>
          <tr>
            <th>Company</th>
            <th>Legal form</th>
            <th>Tax ID</th>
            <th>Contact person</th>
            <th>Phone</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr>
              <td colSpan="6" className="loading-row">
                <div className="loading-spinner">Loading companies…</div>
              </td>
            </tr>
          ) : filteredClients.length === 0 ? (
            <tr>
              <td colSpan="6" className="no-data">
                {filterByUser ? "You have not created any companies yet." : "No companies yet."}
              </td>
            </tr>
          ) : (
            filteredClients.map((client) => (
              <tr key={client.id}>
                <td
                  className="company-name"
                  onClick={() => setSelectedCompanyId(client.id)}
                >
                  {client.name}
                </td>
                <td>{client.form || "—"}</td>
                <td>{client.inn || "—"}</td>
                <td>{client.contact_person || "—"}</td>
                <td>{client.phone || "—"}</td>
                <td>
                  <button
                    onClick={() => {
                      setEditingClient(client);
                      setShowForm(true);
                    }}
                    className="icon-btn edit-btn"
                    title="Edit company"
                    aria-label="Edit company"
                  >
                    <EditIcon />
                  </button>
                  <button
                    onClick={() => handleDelete(client.id)}
                    className="icon-btn delete-btn"
                    title="Delete company"
                    aria-label="Delete company"
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

export default ClientList;
