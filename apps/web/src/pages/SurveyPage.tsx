import { SurveyForm } from "../features/survey/SurveyForm";

export default function SurveyPage() {
  return (
    <article className="page registration-page">
      <div className="eyebrow">Pesquisa voluntária · Fase 3</div>
      <h1 data-route-heading tabIndex={-1}>
        Pesquisa turística
      </h1>
      <SurveyForm />
    </article>
  );
}
