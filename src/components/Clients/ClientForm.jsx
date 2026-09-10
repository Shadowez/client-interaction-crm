import { useState } from "react";
import { supabase } from "../../lib/supabaseClient";
import "../../styles/ClientForm.css";

const clean = (value) => value.trim();

const normalizeWebsite = (value) => {
  const trimmed = clean(value);
  if (!trimmed) return null;
  const candidate = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
  return new URL(candidate).toString();
};

const ClientForm = ({ client, onSave, onCancel }) => {
  const [name, setName] = useState(client?.name || "");
  const [form, setForm] = useState(client?.form || "");
  const [inn, setInn] = useState(client?.inn || "");
  const [contactPerson, setContactPerson] = useState(client?.contact_person || "");
  const [address, setAddress] = useState(client?.address || "");
  const [website, setWebsite] = useState(client?.website || "");
  const [phone, setPhone] = useState(client?.phone || "");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (event) => {
    event.preventDefault();
    setError("");

    const companyName = clean(name);
    if (!companyName) {
      setError("Company name is required.");
      return;
    }

    let normalizedWebsite = null;
    try {
      normalizedWebsite = normalizeWebsite(website);
    } catch {
      setError("Enter a valid website address, for example example.com.");
      return;
    }

    const clientData = {
      name: companyName,
      form: clean(form) || null,
      inn: clean(inn) || null,
      contact_person: clean(contactPerson) || null,
      address: clean(address) || null,
      website: normalizedWebsite,
      phone: clean(phone) || null,
    };

    setLoading(true);
    try {
      const query = client
        ? supabase.from("clients").update(clientData).eq("id", client.id)
        : supabase.from("clients").insert(clientData);
      const { error: saveError } = await query;
      if (saveError) throw saveError;
      onSave();
    } catch (err) {
      console.error("Unable to save company:", err);
      setError(err?.message || "Unable to save the company.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="form-overlay">
      <div className="form-container">
        <h2>{client ? "Edit company" : "Add company"}</h2>
        {error && <div className="form-error">{error}</div>}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="company-name">Company name *</label>
            <input
              id="company-name"
              type="text"
              value={name}
              onChange={(event) => setName(event.target.value)}
              maxLength={200}
              required
              disabled={loading}
            />
          </div>
          <div className="form-group">
            <label htmlFor="legal-form">Legal form</label>
            <input
              id="legal-form"
              type="text"
              value={form}
              onChange={(event) => setForm(event.target.value)}
              placeholder="Ltd., LLC, GmbH, etc."
              maxLength={100}
              disabled={loading}
            />
          </div>
          <div className="form-group">
            <label htmlFor="tax-id">Tax ID</label>
            <input
              id="tax-id"
              type="text"
              value={inn}
              onChange={(event) => setInn(event.target.value)}
              maxLength={100}
              disabled={loading}
            />
          </div>
          <div className="form-group">
            <label htmlFor="contact-person">Contact person</label>
            <input
              id="contact-person"
              type="text"
              value={contactPerson}
              onChange={(event) => setContactPerson(event.target.value)}
              maxLength={200}
              disabled={loading}
            />
          </div>
          <div className="form-group">
            <label htmlFor="address">Address</label>
            <input
              id="address"
              type="text"
              value={address}
              onChange={(event) => setAddress(event.target.value)}
              maxLength={500}
              disabled={loading}
            />
          </div>
          <div className="form-group">
            <label htmlFor="website">Website</label>
            <input
              id="website"
              type="text"
              value={website}
              onChange={(event) => setWebsite(event.target.value)}
              placeholder="example.com"
              maxLength={500}
              disabled={loading}
            />
          </div>
          <div className="form-group">
            <label htmlFor="phone">Phone</label>
            <input
              id="phone"
              type="tel"
              value={phone}
              onChange={(event) => setPhone(event.target.value)}
              maxLength={100}
              disabled={loading}
            />
          </div>
          <div className="form-buttons">
            <button type="submit" disabled={loading}>
              {loading ? "Saving…" : "Save"}
            </button>
            <button type="button" onClick={onCancel} disabled={loading}>
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default ClientForm;
