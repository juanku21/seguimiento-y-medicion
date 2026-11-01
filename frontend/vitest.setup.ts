import { configure } from "@testing-library/react";

// Con jsdom y recharts, el primer render de un archivo de pruebas puede superar el segundo que
// esperan por defecto las consultas find*. Se amplía la espera para no fallar por tiempos del
// entorno; una consulta que nunca se cumple sigue fallando.
configure({ asyncUtilTimeout: 5000 });
