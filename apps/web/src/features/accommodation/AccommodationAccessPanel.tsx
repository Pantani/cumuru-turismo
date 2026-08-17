import { useEffect, useState } from "react";

import type { Accommodation } from "../operator/stay-lifecycle";
import { ActivationIssuePanel } from "./ActivationIssuePanel";
import { PosterInvitePanel } from "./PosterInvitePanel";

interface AccommodationAccessPanelProps {
  accommodation: Accommodation;
}

/**
 * Both panels write commands guarded by `If-Match` against the same
 * accommodation row, so the row version lives here, in one place. A version
 * kept twice would make the second command fail a precondition the operator
 * has no way to understand.
 */
export function AccommodationAccessPanel({
  accommodation,
}: AccommodationAccessPanelProps) {
  const [version, setVersion] = useState(accommodation.version);

  useEffect(() => {
    setVersion(accommodation.version);
  }, [accommodation.id, accommodation.version]);

  return (
    <div className="accommodation-access">
      <PosterInvitePanel
        accommodationId={accommodation.id}
        onVersionChange={setVersion}
        version={version}
      />
      <ActivationIssuePanel
        accommodationId={accommodation.id}
        onVersionChange={setVersion}
        version={version}
      />
    </div>
  );
}
