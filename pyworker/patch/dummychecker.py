class DummySafetyChecker():
    def safety_checker(self, images, *args, **kwargs):
        return images, [False] * len(images)
