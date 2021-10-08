textures/water
	{
	water
	qer_editorimage textures/w1.jpg
	surfaceparm trans
	surfaceparm nomarks
	surfaceparm nolightmap
	cull none
	{
		animMap 10 textures/w1.jpg textures/w2.jpg textures/w3.jpg textures/w4.jpg
		blendFunc GL_ONE GL_ONE
		rgbGen wave inverseSawtooth 0 1 0 10
	}
}