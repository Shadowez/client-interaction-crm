import { useState } from "react";
import branding from "../../config/branding.json";
import "../../styles/BrandMark.css";

const getDisplayName = () =>
  branding.organizationName?.trim() || branding.productName?.trim() || "CRM";

const getLogoUrl = () => {
  if (!branding.logoPath?.trim()) return "";
  const relativePath = branding.logoPath.replace(/^\/+/, "");
  return `${import.meta.env.BASE_URL}${relativePath}`;
};

const BrandMark = ({ showName = true, compact = false }) => {
  const [logoFailed, setLogoFailed] = useState(false);
  const displayName = getDisplayName();
  const logoUrl = getLogoUrl();
  const initial = Array.from(displayName)[0]?.toUpperCase() || "C";

  return (
    <div className={`brand-mark ${compact ? "brand-mark-compact" : ""}`}>
      {logoUrl && !logoFailed ? (
        <img
          src={logoUrl}
          alt={`${displayName} logo`}
          className="brand-mark-logo"
          onError={() => setLogoFailed(true)}
        />
      ) : (
        <div className="brand-mark-monogram" aria-hidden="true">
          {initial}
        </div>
      )}
      {showName && <span className="brand-mark-name">{displayName}</span>}
    </div>
  );
};

export default BrandMark;
