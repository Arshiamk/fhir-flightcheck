import { OperationsConsole } from "@/components/operations-console";
import { createRepository } from "@/lib/repository";

export default async function Home() {
  const repository = createRepository();
  const isLive = !!(process.env.CONTROL_PLANE_URL ?? process.env.NEXT_PUBLIC_CONTROL_PLANE_URL);
  const data = await repository.getOperationsSnapshot();
  return <OperationsConsole data={data} isLive={isLive} />;
}
