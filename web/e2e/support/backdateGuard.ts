// Safety rule for the smoke specs that move payment times back in the database. They may do it only in the stack that
// scripts/smoke.sh started (it exports that stack's compose project as SMOKE_COMPOSE_PROJECT; `make demo-check` runs
// `make smoke` with its own project, so that works too) and only in a guesthouse made by that run (code "smoke…").
// `talkedProject` is the compose project the database container really belongs to (docker label), so a wrong variable
// cannot send the change to another stack. A rehearsal stack is refused whatever the variable says.
export function backdateRefusal(
  env: Record<string, string | undefined>,
  talkedProject: string,
): string | null {
  const project = env.SMOKE_COMPOSE_PROJECT ?? "";
  if (!project) return "SMOKE_COMPOSE_PROJECT is not set: run this through `make smoke`";
  if (/^stayguard-rehearse/.test(project) || /^stayguard-rehearse/.test(talkedProject))
    return "never in a rehearsal stack";
  if (talkedProject !== project)
    return `the database belongs to "${talkedProject}", not the one this run started ("${project}")`;
  if (!/^smoke/.test(env.SMOKE_GUESTHOUSE ?? ""))
    return `guesthouse "${env.SMOKE_GUESTHOUSE}" was not made by a smoke run`;
  return null;
}
