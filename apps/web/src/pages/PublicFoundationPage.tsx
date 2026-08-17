import { AnalyticsDashboard } from "../features/analytics/AnalyticsDashboard";
import { AboutSection } from "../features/landing/AboutSection";
import { CommerceSection } from "../features/landing/CommerceSection";
import { ContactSection } from "../features/landing/ContactSection";
import { HeroSection } from "../features/landing/HeroSection";
import { HostsSection } from "../features/landing/HostsSection";
import { HowItWorksSection } from "../features/landing/HowItWorksSection";
import { PlaceSection } from "../features/landing/PlaceSection";
import { PrivacySection } from "../features/landing/PrivacySection";
import { SectionNav } from "../features/landing/SectionNav";
import { TickerBar } from "../features/landing/TickerBar";
import {
  publicAnalyticsClient,
  type AnalyticsClient,
} from "../shared/api/analytics-client";

/**
 * Capa pública do Observatório.
 *
 * A ordem é deliberada: o número vem antes do argumento. Quem chega pelo QR de
 * uma pousada vê a presença de hoje, depois o painel completo e só então o
 * convite para participar. Todo indicador desta página sai do contrato público
 * já agregado e arredondado — a capa não tem caminho para microdado.
 */
export default function PublicFoundationPage({
  client = publicAnalyticsClient,
}: {
  client?: AnalyticsClient;
}) {
  return (
    <article className="landing">
      <SectionNav />
      <HeroSection client={client} />
      <TickerBar client={client} />
      <section className="lp-section lp-section-deep" id="numeros">
        <div className="lp-shell">
          <AnalyticsDashboard client={client} />
        </div>
      </section>
      <HowItWorksSection />
      <HostsSection />
      <CommerceSection />
      <PlaceSection />
      <PrivacySection />
      <AboutSection />
      <ContactSection />
    </article>
  );
}
