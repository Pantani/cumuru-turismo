package config

// firstError runs the checks in order and returns the first failure, so a
// validator reads as a list of named rules instead of a chain of branches.
func firstError(checks ...func() error) error {
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

// localOrTest gates every prototype-only feature: none of them may switch on in
// a deployed environment.
func localOrTest(environment Environment) bool {
	return environment == EnvironmentLocal || environment == EnvironmentTest
}
