"use client";

import { useRouter } from "next/navigation";
import { Suspense } from "react";
import { DefectForm } from "@/features/defects/components/defect-form";

// Vista de la pantalla de alta de un defecto (007/US1). Al crear el defecto lleva a su detalle. El
// formulario lee ?storyId= con useSearchParams, así que va dentro de Suspense para que el resto de
// la página se siga prerenderizando.
export function NewDefectView({ projectId }: { projectId: string }) {
  const router = useRouter();
  return (
    <Suspense>
      <DefectForm
        projectId={projectId}
        onCreated={(defect) =>
          router.push(`/projects/${projectId}/defects/${defect.id}`)
        }
      />
    </Suspense>
  );
}
